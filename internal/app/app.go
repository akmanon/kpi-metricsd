package app

import (
	"context"
	"fmt"
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
	logger          *zap.Logger
}

func New(cfg *config.Cfg, logger *zap.Logger) (*App, error) {

	srcFile := cfg.LogCfg.SourceLogFile
	destFile := cfg.LogCfg.RedirectLogFile
	destFileRotated := cfg.LogCfg.RotatedLogFile
	rotateInt, _ := time.ParseDuration(cfg.LogCfg.RotationInterval)

	logTail := logtail.NewTailAndRedirect(srcFile, destFile, logger)
	logRotate := logrotate.NewLogRotate(destFile, destFileRotated, rotateInt, logger)
	logMetrics, err := logmetrics.NewLogMetrics(cfg, destFileRotated, logger)
	if err != nil {
		logger.Error("", zap.Error(err))
		return nil, err
	}
	return &App{
		LogRotate:       logRotate,
		TailAndRedirect: logTail,
		LogMetrics:      logMetrics,
		logger:          logger,
	}, nil
}

func (app *App) Run(ctx context.Context) error {
	var wg sync.WaitGroup
	rotateChan := make(chan bool)
	processMetricsNotifyCh := make(chan bool)
	errChan := make(chan error, 3)

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := app.LogRotate.Start(rotateChan, processMetricsNotifyCh); err != nil {
			errChan <- fmt.Errorf("logRotate failed %w", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := app.TailAndRedirect.Start(rotateChan); err != nil {
			errChan <- fmt.Errorf("tailing failed %w", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := app.LogMetrics.Start(processMetricsNotifyCh); err != nil {
			errChan <- fmt.Errorf("metrics failed %w", err)
		}
	}()

	select {
	case <-ctx.Done():
		app.logger.Info("context cancelled, shutting down...")
	case err := <-errChan:
		app.logger.Error("application error", zap.Error(err))
		return err
	}

	app.logger.Info("shutting down components")
	app.Stop()

	wg.Wait()

	return nil
}

func (app *App) Stop() {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		app.LogRotate.Stop()
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		app.TailAndRedirect.Stop()
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		app.LogMetrics.Stop()
	}()
	wg.Wait()
}
