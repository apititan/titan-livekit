package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/spf13/viper"
	"go.opentelemetry.io/contrib/instrumentation/github.com/labstack/echo/otelecho"
	jaegerPropagator "go.opentelemetry.io/contrib/propagators/jaeger"
	"go.opentelemetry.io/otel"
	jaegerExporter "go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/fx"
	"net/http"
	"nkonev.name/video/client"
	"nkonev.name/video/config"
	"nkonev.name/video/handlers"
	. "nkonev.name/video/logger"
	"nkonev.name/video/producer"
	"nkonev.name/video/rabbitmq"
	"nkonev.name/video/redis"
	"nkonev.name/video/services"
)

const EXTERNAL_TRACE_ID_HEADER = "trace-id"
const TRACE_RESOURCE = "video"

func main() {
	config.InitViper()

	app := fx.New(
		fx.Logger(Logger),
		fx.Provide(
			createTypedConfig,
			configureTracer,
			configureApiEcho,
			configureEgressLayoutEcho,
			client.NewRestClient,
			client.NewLivekitClient,
			client.NewEgressClient,
			handlers.NewUserHandler,
			handlers.NewConfigHandler,
			handlers.ConfigureApiStaticMiddleware,
			handlers.ConfigureEgressLayoutStaticMiddleware,
			handlers.ConfigureAuthMiddleware,
			handlers.NewTokenHandler,
			handlers.NewLivekitWebhookHandler,
			handlers.NewInviteHandler,
			handlers.NewRecordHandler,
			rabbitmq.CreateRabbitMqConnection,
			producer.NewRabbitUserCountPublisher,
			producer.NewRabbitInvitePublisher,
			producer.NewRabbitDialStatusPublisher,
			producer.NewRabbitRecordingPublisher,
			services.NewNotificationService,
			services.NewUserService,
			services.NewStateChangedNotificationService,
			services.NewDialRedisRepository,
			services.NewEgressService,
			redis.RedisV8,
			redis.NewVideoCallUsersCountNotifierService,
			redis.VideoCallUsersCountNotifierScheduler,
			redis.NewChatDialerService,
			redis.ChatDialerScheduler,
			redis.NewRecordingNotifierService,
			redis.RecordingNotifierScheduler,
		),
		fx.Invoke(
			runApiEcho,
			runEgressLayoutEcho,
			runScheduler,
		),
	)
	app.Run()

	Logger.Infof("Exit program")
}

func configureWriteHeaderMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx echo.Context) (err error) {
			handler := http.HandlerFunc(
				func(w http.ResponseWriter, r *http.Request) {
					ctx.SetRequest(r)
					ctx.SetResponse(echo.NewResponse(w, ctx.Echo()))
					existsSpan := trace.SpanFromContext(ctx.Request().Context())
					if existsSpan.SpanContext().HasSpanID() {
						w.Header().Set(EXTERNAL_TRACE_ID_HEADER, existsSpan.SpanContext().TraceID().String())
					}
					err = next(ctx)
				},
			)
			handler.ServeHTTP(ctx.Response(), ctx.Request())
			return
		}
	}
}

func configureOpentelemetryMiddleware(tp *sdktrace.TracerProvider) echo.MiddlewareFunc {
	mw := otelecho.Middleware(TRACE_RESOURCE, otelecho.WithTracerProvider(tp))
	return mw
}

func createCustomHTTPErrorHandler(e *echo.Echo) func(err error, c echo.Context) {
	originalHandler := e.DefaultHTTPErrorHandler
	return func(err error, c echo.Context) {
		GetLogEntry(c.Request().Context()).Errorf("Unhandled error: %v", err)
		originalHandler(err, c)
	}
}

type ApiEcho struct {
	*echo.Echo
}

type EgressLayoutEcho struct {
	*echo.Echo
}

func configureApiEcho(
	cfg *config.ExtendedConfig,
	authMiddleware handlers.AuthMiddleware,
	staticMiddleware handlers.ApiStaticMiddleware,
	lc fx.Lifecycle,
	th *handlers.TokenHandler,
	uh *handlers.UserHandler,
	ch *handlers.ConfigHandler,
	lhf *handlers.LivekitWebhookHandler,
	ih *handlers.InviteHandler,
	rh *handlers.RecordHandler,
	tp *sdktrace.TracerProvider,
) *ApiEcho {

	bodyLimit := cfg.HttpServerConfig.BodyLimit

	e := echo.New()
	e.Logger.SetOutput(Logger.Writer())

	e.HTTPErrorHandler = createCustomHTTPErrorHandler(e)

	e.Pre(echo.MiddlewareFunc(staticMiddleware))
	e.Use(configureOpentelemetryMiddleware(tp))
	e.Use(configureWriteHeaderMiddleware())
	e.Use(echo.MiddlewareFunc(authMiddleware))
	accessLoggerConfig := middleware.LoggerConfig{
		Output: Logger.Writer(),
		Format: `"remote_ip":"${remote_ip}",` +
			`"method":"${method}","uri":"${uri}",` +
			`"status":${status},` +
			`,"bytes_in":${bytes_in},"bytes_out":${bytes_out},"traceId":"${header:uber-trace-id}"` + "\n",
	}
	e.Use(middleware.LoggerWithConfig(accessLoggerConfig))
	e.Use(middleware.Secure())
	e.Use(middleware.BodyLimit(bodyLimit))

	e.GET("/video/:chatId/token", th.GetTokenHandler)
	e.GET("/video/:chatId/users", uh.GetVideoUsers)
	e.GET("/video/:chatId/config", ch.GetConfig)
	e.POST("/internal/livekit-webhook", lhf.GetLivekitWebhookHandler())
	e.PUT("/video/:chatId/kick", uh.Kick)
	e.PUT("/video/:chatId/mute", uh.Mute)
	e.PUT("/video/:id/dial", ih.ProcessCallInvitation)
	e.PUT("/video/:id/dial/cancel", ih.ProcessCancelInvitation)
	e.PUT("/video/:id/dial/stop", ih.ProcessAsOwnerLeave)
	e.PUT("/video/:id/record/start", rh.StartRecording)
	e.PUT("/video/:id/record/stop", rh.StopRecording)
	e.GET("/video/:id/record/status", rh.StatusRecording)

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			// do some work on application stop (like closing connections and files)
			Logger.Infof("Stopping http server")
			return e.Shutdown(ctx)
		},
	})

	return &ApiEcho{e}
}

func configureEgressLayoutEcho(
	staticMiddleware handlers.EgressLayoutStaticMiddleware,
	lc fx.Lifecycle,
) *EgressLayoutEcho {
	e := echo.New()
	e.Logger.SetOutput(Logger.Writer())

	e.HTTPErrorHandler = createCustomHTTPErrorHandler(e)

	accessLoggerConfig := middleware.LoggerConfig{
		Output: Logger.Writer(),
		Format: `"remote_ip":"${remote_ip}",` +
			`"method":"${method}","uri":"${uri}",` +
			`"status":${status},` +
			`,"bytes_in":${bytes_in},"bytes_out":${bytes_out},"traceId":"${header:uber-trace-id}"` + "\n",
	}
	e.Use(middleware.LoggerWithConfig(accessLoggerConfig))
	e.Use(echo.MiddlewareFunc(staticMiddleware))

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			// do some work on application stop (like closing connections and files)
			Logger.Infof("Stopping http server")
			return e.Shutdown(ctx)
		},
	})

	return &EgressLayoutEcho{e}
}

func configureTracer(lc fx.Lifecycle, cfg *config.ExtendedConfig) (*sdktrace.TracerProvider, error) {
	Logger.Infof("Configuring Jaeger tracing")
	endpoint := jaegerExporter.WithAgentEndpoint(
		jaegerExporter.WithAgentHost(cfg.JaegerConfig.Host),
		jaegerExporter.WithAgentPort(cfg.JaegerConfig.Port),
	)
	exporter, err := jaegerExporter.New(endpoint)
	if err != nil {
		return nil, err
	}
	resources := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceNameKey.String(TRACE_RESOURCE),
	)
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resources),
	)
	otel.SetTracerProvider(tp)
	jaeger := jaegerPropagator.Jaeger{}
	// register jaeger propagator
	otel.SetTextMapPropagator(jaeger)
	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			Logger.Infof("Stopping tracer")
			if err := tp.Shutdown(context.Background()); err != nil {
				Logger.Printf("Error shutting down tracer provider: %v", err)
			}
			return nil
		},
	})

	return tp, nil
}

// rely on viper import and it's configured by
func runApiEcho(e *ApiEcho, cfg *config.ExtendedConfig) {
	address := cfg.HttpServerConfig.ApiAddress

	Logger.Info("Starting api server...")
	// Start server in another goroutine
	go func() {
		if err := e.Start(address); err != nil {
			Logger.Infof("server shut down: %v", err)
		}
	}()
	Logger.Info("Api server started. Waiting for interrupt signal 2 (Ctrl+C)")
}

func runEgressLayoutEcho(e *EgressLayoutEcho, cfg *config.ExtendedConfig) {
	address := cfg.HttpServerConfig.EgressLayoutAddress

	Logger.Info("Starting egress layout server...")
	// Start server in another goroutine
	go func() {
		if err := e.Start(address); err != nil {
			Logger.Infof("server shut down: %v", err)
		}
	}()
	Logger.Info("Egress layout server started. Waiting for interrupt signal 2 (Ctrl+C)")
}

func runScheduler(chatNotifierTask *redis.VideoCallUsersCountNotifierTask, chatDialerTask *redis.ChatDialerTask, videoRecordingTask *redis.RecordingNotifierTask) {
	go func() {
		err := chatNotifierTask.Run(context.Background())
		if err != nil {
			Logger.Errorf("Error during working videoUsersCountNotifierTask: %s", err)
		}
	}()
	go func() {
		err := chatDialerTask.Run(context.Background())
		if err != nil {
			Logger.Errorf("Error during working chatDialerTask: %s", err)
		}
	}()
	go func() {
		err := videoRecordingTask.Run(context.Background())
		if err != nil {
			Logger.Errorf("Error during working chatDialerTask: %s", err)
		}
	}()

	Logger.Infof("Schedulers are started")
}

func createTypedConfig() (*config.ExtendedConfig, error) {
	conf := config.ExtendedConfig{}
	err := viper.GetViper().Unmarshal(&conf)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("sfu extended config file loaded failed. %v\n", err))
	}

	return &conf, nil
}
