package logmetrics

import (
	"bufio"
	"context"
	"fmt"
	"maps"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/akmanon/kpi-metricsd/internal/config"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/client_golang/prometheus/push"
	"go.uber.org/zap"
)

type LogMetrics struct {
	logFile        string
	kpis           *[]config.KPI
	compiledRegex  map[string]*regexp.Regexp
	ctx            context.Context
	cancel         context.CancelFunc
	promMetrics    map[string]prometheus.Gauge
	kpiCount       map[string]float64
	logger         *zap.Logger
	mu             sync.Mutex
	listenAddr     string
	metricsPath    string
	PushGatewayCfg config.PushGateway
}

func NewLogMetrics(cfg *config.Cfg, logFile string, logger *zap.Logger) (*LogMetrics, error) {
	ctx, cancel := context.WithCancel(context.Background())
	compiledRegex := make(map[string]*regexp.Regexp)
	promMetrics := make(map[string]prometheus.Gauge)
	kpiCount := make(map[string]float64)
	var pushGatewayCfg config.PushGateway
	kpis := &cfg.KPIs
	fmt.Println(cfg.Server.PushGateway)
	if cfg.Server.PushGateway.Enabled {
		pushGatewayCfg = cfg.Server.PushGateway
	}

	err := compileRegexpFromCfg(kpis, &compiledRegex)
	if err != nil {
		logger.Error("failed to compile regex", zap.Error(err))
		cancel()
		return nil, err
	}
	logger.Info("regex from config has been compiled sucessfully")
	return &LogMetrics{
		kpis:           kpis,
		compiledRegex:  compiledRegex,
		promMetrics:    promMetrics,
		logFile:        logFile,
		kpiCount:       kpiCount,
		ctx:            ctx,
		cancel:         cancel,
		logger:         logger,
		listenAddr:     ":" + strconv.Itoa(cfg.Server.Port),
		metricsPath:    cfg.Server.MetricsPath,
		PushGatewayCfg: pushGatewayCfg,
	}, nil
}

func compileRegexpFromCfg(kpis *[]config.KPI, compiledRegex *map[string]*regexp.Regexp) error {
	for _, kpi := range *kpis {
		if kpi.Regex == "" {
			continue
		}
		re, err := regexp.Compile(kpi.Regex)
		if err != nil {
			return err
		}
		(*compiledRegex)[kpi.Name] = re
	}
	return nil
}

func (lm *LogMetrics) Start(metricsChan <-chan bool) error {

	lm.initMetrics()

	go lm.serveMetrics()

	for {
		select {
		case <-metricsChan:
			err := lm.updatePromMetrics()
			if err != nil {
				return err
			}
		case <-lm.ctx.Done():
			return lm.ctx.Err()
		}
	}
}

func (lm *LogMetrics) initMetrics() {
	var constLabels prometheus.Labels
	for _, kpi := range *lm.kpis {

		if _, exists := lm.promMetrics[kpi.Name]; exists {
			continue
		}

		constLabels = make(prometheus.Labels)
		if len(kpi.CustomLabels) > 0 {
			maps.Copy(constLabels, kpi.CustomLabels)
		}
		gauge := promauto.NewGauge(prometheus.GaugeOpts{
			Name:        kpi.Name,
			Help:        "count of " + kpi.Name + " events from log monitoring",
			ConstLabels: constLabels,
		})
		lm.promMetrics[kpi.Name] = gauge
	}
}

func (lm *LogMetrics) updatePromMetrics() error {
	err := lm.updateKPICount()
	if err != nil {
		return err
	}

	for k, v := range lm.kpiCount {
		lm.promMetrics[k].Set(v)
	}
	if lm.PushGatewayCfg.Enabled {
		lm.pushMetrics()
	}
	return nil

}

func (lm *LogMetrics) pushMetrics() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pusher := push.New(lm.PushGatewayCfg.URL, lm.PushGatewayCfg.Job).
		Grouping("instance", lm.PushGatewayCfg.Instance)

	// Push all KPIs
	for _, metric := range lm.promMetrics {
		pusher.Collector(metric)
	}

	if err := pusher.PushContext(ctx); err != nil {
		lm.logger.Info("failed to push metrics to PushGateway", zap.Error(err))
	} else {
		lm.logger.Info("metrics pushed to PushGateway")
	}
	return nil

}

func (lm *LogMetrics) serveMetrics() {
	mux := http.NewServeMux()
	mux.Handle(lm.metricsPath, promhttp.Handler())

	if err := http.ListenAndServe(lm.listenAddr, mux); err != nil {
		lm.logger.Fatal("metrics server failed", zap.Error(err))
	}
}

func (lm *LogMetrics) Stop() {
	lm.logger.Info("stopping metrics component")
	lm.cancel()
}

func (lm *LogMetrics) updateKPICount() error {

	lm.mu.Lock()
	defer lm.mu.Unlock()

	lm.logger.Info("Triggered KPI count update")
	lm.resetKPICount()

	f, err := os.Open(lm.logFile)
	if err != nil {
		lm.logger.Error("failed to open rotated log file", zap.Error(err))
		return fmt.Errorf("failed to open rotated log file %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	buf := make([]byte, 0, 1024*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		for kpiName, re := range lm.compiledRegex {
			if re.MatchString(scanner.Text()) {
				lm.kpiCount[kpiName]++
			}
		}
	}
	if err := scanner.Err(); err != nil {
		lm.logger.Error("scanner error while reading rotated log file", zap.Error(err))
		return nil
	}
	lm.logger.Info("KPI count update completed")
	return nil
}

func (lm *LogMetrics) resetKPICount() {
	for kpiName := range lm.compiledRegex {
		lm.kpiCount[kpiName] = 0
	}
}
