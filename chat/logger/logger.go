package logger

import (
	log "github.com/sirupsen/logrus"
	"os"
)

var Logger = log.New()

func init() {
	Logger.SetReportCaller(true)
	Logger.SetFormatter(&log.TextFormatter{ForceColors: true, FullTimestamp: true})
	Logger.SetOutput(os.Stdout)
}

func GetLogger(traceid string) *log.Entry {
	return Logger.WithFields(
		log.Fields{
			"traceId": traceid,
		})
}
