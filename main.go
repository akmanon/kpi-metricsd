package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/akmanon/kpi-metricsd/internal/app"
	"github.com/akmanon/kpi-metricsd/internal/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {

	loggerCfg := zap.NewProductionConfig()
	loggerCfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	logger, _ := loggerCfg.Build()
	defer logger.Sync()

	cfgPath := flag.String("config", "config.yaml", "Path to yaml config file")
	flag.Parse()

	cfg, err := config.LoadCfg(*cfgPath)
	if err != nil {
		logger.Fatal("failed to load config", zap.Error(err))
	}

	app, err := app.New(cfg, logger)
	if err != nil {
		log.Fatalf("Failed to create application: %v", err)
	}

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		logger.Info("Shutdown signal received")
		cancel()
	}()

	// Run application
	if err := app.Run(ctx); err != nil {
		logger.Fatal("main application failed", zap.Error(err))
	}

	logger.Info("main application stopped")

}
