package main

import (
	"context"
	"contrib.go.opencensus.io/exporter/jaeger"
	"github.com/centrifugal/centrifuge"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/microcosm-cc/bluemonday"
	uberCompat "github.com/nkonev/jaeger-uber-propagation-compat/propagation"
	"github.com/spf13/viper"
	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/trace"
	"go.uber.org/fx"
	"net/http"
	"nkonev.name/chat/client"
	"nkonev.name/chat/db"
	"nkonev.name/chat/handlers"
	"nkonev.name/chat/listener"
	. "nkonev.name/chat/logger"
	"nkonev.name/chat/notifications"
	"nkonev.name/chat/utils"
)

const EXTERNAL_TRACE_ID_HEADER = "trace-id"

func main() {
	configFile := utils.InitFlags("./chat/config-dev/config.yml")
	utils.InitViper(configFile, "CHAT")

	app := fx.New(
		fx.Logger(Logger),
		fx.Provide(
			client.NewRestClient,
			handlers.ConfigureCentrifuge,
			handlers.CreateSanitizer,
			configureEcho,
			handlers.ConfigureStaticMiddleware,
			handlers.ConfigureAuthMiddleware,
			configureMigrations,
			db.ConfigureDb,
			notifications.NewNotifications,
			listener.RedisAaaConnection,
			listener.CreateAaaUserProfileUpdateListener,
		),
		fx.Invoke(
			initJaeger,
			runMigrations,
			runCentrifuge,
			runEcho,
			listener.ListenPubSubChannels,
		),
	)
	app.Run()

	Logger.Infof("Exit program")
}

func runCentrifuge(node *centrifuge.Node) {
	// Run node.
	Logger.Infof("Starting centrifuge...")
	go func() {
		if err := node.Run(); err != nil {
			Logger.Fatalf("Error on start centrifuge: %v", err)
		}
	}()
	Logger.Info("Centrifuge started.")
}

func configureOpencensusMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx echo.Context) (err error) {
			handler := &ochttp.Handler{
				Handler: http.HandlerFunc(
					func(w http.ResponseWriter, r *http.Request) {
						ctx.SetRequest(r)
						ctx.SetResponse(echo.NewResponse(w, ctx.Echo()))
						existsSpan := trace.FromContext(ctx.Request().Context())
						if existsSpan != nil {
							w.Header().Set(EXTERNAL_TRACE_ID_HEADER, existsSpan.SpanContext().TraceID.String())
						}
						err = next(ctx)
					},
				),
				Propagation: &uberCompat.HTTPFormat{},
			}
			handler.ServeHTTP(ctx.Response(), ctx.Request())
			return
		}
	}
}

func createCustomHTTPErrorHandler(e *echo.Echo) func(err error, c echo.Context) {
	originalHandler := e.DefaultHTTPErrorHandler
	return func(err error, c echo.Context) {
		GetLogEntry(c.Request()).Errorf("Unhandled error: %v", err)
		originalHandler(err, c)
	}
}

func configureEcho(
	staticMiddleware handlers.StaticMiddleware,
	authMiddleware handlers.AuthMiddleware,
	lc fx.Lifecycle,
	notificator notifications.Notifications,
	node *centrifuge.Node,
	db db.DB,
	policy *bluemonday.Policy,
	restClient client.RestClient,
) *echo.Echo {

	bodyLimit := viper.GetString("server.body.limit")

	e := echo.New()
	e.Logger.SetOutput(Logger.Writer())

	e.HTTPErrorHandler = createCustomHTTPErrorHandler(e)

	e.Pre(echo.MiddlewareFunc(staticMiddleware))
	e.Use(configureOpencensusMiddleware())
	e.Use(echo.MiddlewareFunc(authMiddleware))
	accessLoggerConfig := middleware.LoggerConfig{
		Output: Logger.Writer(),
		Format: `"remote_ip":"${remote_ip}",` +
			`"method":"${method}","uri":"${uri}",` +
			`"status":${status},` +
			`,"bytes_in":${bytes_in},"bytes_out":${bytes_out},"traceId":"${header:X-B3-Traceid}"` + "\n",
	}
	e.Use(middleware.LoggerWithConfig(accessLoggerConfig))
	e.Use(middleware.Secure())
	e.Use(middleware.BodyLimit(bodyLimit))

	e.GET("/chat/websocket", handlers.Convert(handlers.CentrifugeAuthMiddleware(centrifuge.NewWebsocketHandler(node, centrifuge.WebsocketConfig{}))))

	ch := handlers.NewChatHandler(db, notificator, restClient)
	e.GET("/chat", ch.GetChats)
	e.GET("/chat/:id", ch.GetChat)
	e.POST("/chat", ch.CreateChat)
	e.DELETE("/chat/:id", ch.DeleteChat)
	e.PUT("/chat", ch.EditChat)
	e.PUT("/chat/:id/leave", ch.LeaveChat)
	e.GET("/internal/access", ch.CheckAccess)

	mc := handlers.NewMessageHandler(db, policy, notificator, restClient)
	e.GET("/chat/:id/message", mc.GetMessages)
	e.GET("/chat/:id/message/:messageId", mc.GetMessage)
	e.POST("/chat/:id/message", mc.PostMessage)
	e.PUT("/chat/:id/message", mc.EditMessage)
	e.DELETE("/chat/:id/message/:messageId", mc.DeleteMessage)
	e.PUT("/chat/:id/message/read/:messageId", mc.ReadMessage)
	e.PUT("/chat/:id/typing", mc.TypeMessage)
	e.PUT("/chat/:id/broadcast", mc.BroadcastMessage)

	vh := handlers.NewVideoHandler(db, restClient, notificator)
	e.PUT("/internal/video/notify", vh.NotifyAboutVideoCallChange)
	e.PUT("/internal/video/kick", vh.NotifyAboutKick)
	e.PUT("/chat/:id/video/invite", vh.NotifyAboutCallInvitation)
	e.PUT("/chat/:id/video/kick", vh.Kick)

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			// do some work on application stop (like closing connections and files)
			Logger.Infof("Stopping http server")
			return e.Shutdown(ctx)
		},
	})

	return e
}

func initJaeger(lc fx.Lifecycle) error {
	exporter, err := jaeger.NewExporter(jaeger.Options{
		AgentEndpoint: viper.GetString("jaeger.endpoint"),
		Process: jaeger.Process{
			ServiceName: "chat",
		},
	})
	if err != nil {
		return err
	}
	trace.RegisterExporter(exporter)
	trace.ApplyConfig(trace.Config{
		DefaultSampler: trace.AlwaysSample(),
	})
	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			Logger.Infof("Stopping tracer")
			exporter.Flush()
			trace.UnregisterExporter(exporter)
			return nil
		},
	})
	return nil
}

func configureMigrations() db.MigrationsConfig {
	return db.MigrationsConfig{}
}

func runMigrations(db db.DB, migrationsConfig db.MigrationsConfig) {
	db.Migrate(migrationsConfig)
}

// rely on viper import and it's configured by
func runEcho(e *echo.Echo) {
	address := viper.GetString("server.address")

	Logger.Info("Starting server...")
	// Start server in another goroutine
	go func() {
		if err := e.Start(address); err != nil {
			Logger.Infof("server shut down: %v", err)
		}
	}()
	Logger.Info("Server started. Waiting for interrupt signal 2 (Ctrl+C)")
}
