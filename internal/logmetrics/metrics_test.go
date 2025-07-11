package logmetrics

import (
	"context"
	"errors"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/akmanon/kpi-metricsd/internal/config"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestLogMetrics(t *testing.T) {

	testLine := `Test 1
test 2
test 3`
	logFile := "../testdata/test.log"
	err := os.WriteFile(logFile, []byte(testLine), 0755)
	assert.NoError(t, err, "Expected no error when writing to test log file")

	cfg, err := config.LoadCfg("../../conf.yaml")
	assert.NoError(t, err)

	metrics, err := NewLogMetrics(cfg, logFile, zap.NewNop())
	assert.NoError(t, err, "Expected no error when creating LogMetrics")

	// Check if the compiled regex map is not empty
	assert.NotEmpty(t, metrics.compiledRegex, "Compiled regex map should not be empty")

	// Check if the regex for a specific KPI is compiled correctly
	kpiName := "test1"
	re, exists := metrics.compiledRegex[kpiName]
	assert.True(t, exists, "KPI regex should exist in the compiled map")
	assert.NotNil(t, re, "Compiled regex for KPI should not be nil")

	// Check if the regex matches a test string
	testString := "This is a test1 string"
	assert.True(t, re.MatchString(testString), "Regex should match the test string")

	// Check if the KPI count is returned correctly
}

func TestLogMetricsStart(t *testing.T) {

	loggerCfg := zap.NewProductionConfig()
	loggerCfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	logger, _ := loggerCfg.Build()
	defer logger.Sync()

	logFile := "../testdata/test.log"
	cfg, err := config.LoadCfg("../testdata/conf.yaml")
	assert.NoError(t, err)

	notifyMetrics := make(chan bool)
	lm, err := NewLogMetrics(cfg, logFile, logger)
	assert.NoError(t, err, "Expected no error when creating LogMetrics")

	go func() {
		ticker := time.NewTicker(time.Millisecond * 300)
		defer ticker.Stop()
		for {
			select {
			case <-lm.ctx.Done():
				return
			case <-ticker.C:
				notifyMetrics <- true
			}
		}

	}()

	go func() {
		time.Sleep(time.Millisecond * 500)
		lm.Stop()
	}()

	go func() {
		err = lm.Start(notifyMetrics)
		assert.Equal(t, err, errors.New(context.Canceled.Error()))
	}()

	time.Sleep(time.Millisecond * 400)
	assert.Equal(t, lm.kpiCount["test1"], float64(2))
	assert.Equal(t, lm.kpiCount["test2"], float64(1))
	assert.Equal(t, lm.kpiCount["test3"], float64(0))

	resp, err := http.Get("http://localhost" + lm.listenAddr + lm.metricsPath)
	assert.NoError(t, err)
	assert.Equal(t, resp.StatusCode, 200)
}
