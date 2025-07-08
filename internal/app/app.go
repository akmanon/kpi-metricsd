package app

import (
	"sync"
	"time"

	"github.com/akmanon/kpi-metricsd/internal/config"
	"github.com/akmanon/kpi-metricsd/internal/logrotate"
	"github.com/akmanon/kpi-metricsd/internal/logtail"
	"go.uber.org/zap"
)

type App struct {
	LogRotate       *logrotate.LogRotate
	TailAndRedirect *logtail.TailAndRedirect
	wg              sync.WaitGroup
}

func NewApp(cfg *config.Cfg, logger *zap.Logger) *App {

	srcFile := cfg.LogCfg.SourceLogFile
	destFile := cfg.LogCfg.RedirectLogFile
	destFileRotated := cfg.LogCfg.RotatedLogFile
	rotateInt, _ := time.ParseDuration(cfg.LogCfg.RotationInterval)

	logTail := logtail.NewTailAndRedirect(srcFile, destFile, logger)
	logRotate := logrotate.NewLogRotate(destFile, destFileRotated, rotateInt, logger)
	return &App{
		LogRotate:       logRotate,
		TailAndRedirect: logTail,
	}
}

func (app *App) Run() {
	rotateChan := make(chan bool)
	app.wg.Add(1)
	go app.LogRotate.Start(rotateChan)
	defer app.wg.Done()

	app.wg.Add(1)
	go app.TailAndRedirect.Start(rotateChan)
	defer app.wg.Done()

	app.wg.Wait()

}

func (app *App) Stop() {
	rotateChan := make(chan bool)
	go app.LogRotate.Stop()
	go app.TailAndRedirect.Stop()
	close(rotateChan)

}
