package app

import (
	"sync"
	"time"

	"github.com/akmanon/kpi-metricsd/internal/config"
	"github.com/akmanon/kpi-metricsd/internal/logmetrics"
	"github.com/akmanon/kpi-metricsd/internal/logrotate"
	"github.com/akmanon/kpi-metricsd/internal/logtail"
	"go.uber.org/zap"
)

type App struct {
	LogRotate       *logrotate.LogRotate
	TailAndRedirect *logtail.TailAndRedirect
	LogMetrics      *logmetrics.LogMetrics
	wg              sync.WaitGroup
}

func NewApp(cfg *config.Cfg, logger *zap.Logger) *App {

	srcFile := cfg.LogCfg.SourceLogFile
	destFile := cfg.LogCfg.RedirectLogFile
	destFileRotated := cfg.LogCfg.RotatedLogFile
	rotateInt, _ := time.ParseDuration(cfg.LogCfg.RotationInterval)

	logTail := logtail.NewTailAndRedirect(srcFile, destFile, logger)
	logRotate := logrotate.NewLogRotate(destFile, destFileRotated, rotateInt, logger)
	logMetrics, err := logmetrics.NewLogMetrics(cfg, destFileRotated, logger)
	if err != nil {
		logger.Error("", zap.Error(err))
	}
	return &App{
		LogRotate:       logRotate,
		TailAndRedirect: logTail,
		LogMetrics:      logMetrics,
	}
}

func (app *App) Run() {
	rotateChan := make(chan bool)
	processMetricsNotify := make(chan bool)

	app.wg.Add(1)
	go app.LogRotate.Start(rotateChan, processMetricsNotify)
	defer app.wg.Done()

	app.wg.Add(1)
	go app.TailAndRedirect.Start(rotateChan)
	defer app.wg.Done()

	app.wg.Add(1)
	go app.LogMetrics.Start(processMetricsNotify)
	app.wg.Wait()

}

func (app *App) Stop() {
	rotateChan := make(chan bool)
	go app.LogRotate.Stop()
	go app.TailAndRedirect.Stop()
	close(rotateChan)
}
