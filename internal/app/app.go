package app

import (
	"time"

	"github.com/akmanon/kpi-metricsd/internal/logrotate"
	"github.com/akmanon/kpi-metricsd/internal/logtail"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type App struct {
	LogRotate       *logrotate.LogRotate
	TailAndRedirect *logtail.TailAndRedirect
}

func NewApp() *App {

	loggerCfg := zap.NewProductionConfig()
	loggerCfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	logger, _ := loggerCfg.Build()
	defer logger.Sync()

	srcFile := "test_log/app.log"
	destFile := "test_log/app_redirect.log"
	rotateInt := time.Second * 10

	logTail := logtail.NewTailAndRedirect(srcFile, destFile, logger)
	logRotate := logrotate.NewLogRotate(srcFile, destFile, rotateInt)
	return &App{
		LogRotate:       logRotate,
		TailAndRedirect: logTail,
	}
}

func (app *App) Run() {
	go app.LogRotate.Start()
	go app.TailAndRedirect.Start()

}
