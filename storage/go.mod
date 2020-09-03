module nkonev.name/storage

require (
	contrib.go.opencensus.io/exporter/jaeger v0.2.0
	github.com/araddon/dateparse v0.0.0-20200409225146-d820a6159ab1
	github.com/disintegration/imaging v1.6.2
	github.com/golang-migrate/migrate/v4 v4.11.0
	github.com/jackc/pgx/v4 v4.8.1
	github.com/labstack/echo/v4 v4.1.16
	github.com/labstack/gommon v0.3.0
	github.com/minio/minio-go/v7 v7.0.4
	github.com/nkonev/jaeger-uber-propagation-compat v0.0.0-20200708125206-e763f0a72519
	github.com/rakyll/statik v0.1.7
	github.com/sirupsen/logrus v1.6.0
	github.com/spf13/viper v1.7.0
	github.com/stretchr/testify v1.5.1
	go.opencensus.io v0.22.4
	go.uber.org/fx v1.12.0
)

go 1.13
