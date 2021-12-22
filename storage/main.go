package main

import (
	"context"
	"contrib.go.opencensus.io/exporter/jaeger"
	"fmt"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	uberCompat "github.com/nkonev/jaeger-uber-propagation-compat/propagation"
	"github.com/spf13/viper"
	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/trace"
	"go.uber.org/fx"
	"net/http"
	"nkonev.name/storage/client"
	"nkonev.name/storage/config"
	"nkonev.name/storage/handlers"
	. "nkonev.name/storage/logger"
	"nkonev.name/storage/utils"
)

const EXTERNAL_TRACE_ID_HEADER = "trace-id"

func main() {
	config.InitViper()

	app := fx.New(
		fx.Logger(Logger),
		fx.Provide(
			configureMinio,
			configureMinioBuckets,
			configureEcho,
			client.NewChatAccessClient,
			handlers.ConfigureStaticMiddleware,
			handlers.ConfigureAuthMiddleware,
			handlers.NewUserAvatarHandler,
			handlers.NewChatAvatarHandler,
			handlers.NewFilesHandler,
			handlers.NewEmbedHandler,
		),
		fx.Invoke(
			initJaeger,
			runEcho,
		),
	)
	app.Run()

	Logger.Infof("Exit program")
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
	uah *handlers.UserAvatarHandler,
	cha *handlers.ChatAvatarHandler,
	fh *handlers.FilesHandler,
	eh *handlers.EmbedHandler,
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

	e.POST("/storage/avatar", uah.PutAvatar)
	e.GET(fmt.Sprintf("%v/:filename", uah.GetUrlPath()), uah.Download)
	e.POST("/storage/chat/:chatId/avatar", cha.PutAvatar)
	e.GET(fmt.Sprintf("%v/:filename", cha.GetUrlPath()), cha.Download)
	e.POST("/storage/:chatId/file", fh.UploadHandler)
	e.POST("/storage/:chatId/file/:fileItemUuid", fh.UploadHandler)
	e.PUT("/storage/:chatId/replace/file", fh.ReplaceHandler)
	e.GET("/storage/:chatId", fh.ListHandler)
	e.DELETE("/storage/:chatId/file", fh.DeleteHandler)
	e.GET("/storage/download", fh.DownloadHandler)
	e.GET(handlers.UrlStorageGetFile, fh.PublicDownloadHandler)
	e.PUT("/storage/publish/file", fh.SetPublic)
	e.GET("/storage/:chatId/file/count/:fileItemUuid", fh.CountHandler)
	e.GET("/storage/:chatId/file", fh.LimitsHandler)
	e.POST("/storage/:chatId/embed", eh.UploadHandler)
	e.GET("/storage/:chatId/embed/:file", eh.DownloadHandler)

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			// do some work on application stop (like closing connections and files)
			Logger.Infof("Stopping http server")
			return e.Shutdown(ctx)
		},
	})

	return e
}

func configureMinio() (*minio.Client, error) {
	endpoint := viper.GetString("minio.endpoint")
	accessKeyID := viper.GetString("minio.accessKeyId")
	secretAccessKey := viper.GetString("minio.secretAccessKey")

	// Initialize minio client object.
	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: false,
	})
	if err != nil {
		return nil, err
	}

	return minioClient, nil
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

func configureMinioBuckets(client *minio.Client) (*utils.MinioConfig, error) {
	var ua, ca, f, e string
	var err error
	if ua, err = utils.EnsureAndGetUserAvatarBucket(client); err != nil {
		return nil, err
	}
	if ca, err = utils.EnsureAndGetChatAvatarBucket(client); err != nil {
		return nil, err
	}
	if f, err = utils.EnsureAndGetFilesBucket(client); err != nil {
		return nil, err
	}
	if e, err = utils.EnsureAndGetEmbeddedBucket(client); err != nil {
		return nil, err
	}
	return &utils.MinioConfig{
		UserAvatar: ua,
		ChatAvatar: ca,
		Files:      f,
		Embedded:   e,
	}, nil
}
