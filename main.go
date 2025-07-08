package main

import (
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

	cfgPath := "conf.yaml"
	cfg, err := config.LoadCfg(cfgPath)
	if err != nil {
		logger.Fatal("failed to load config", zap.Error(err))
	}

	app := app.NewApp(cfg, logger)
	app.Run()
}
