package logrotate

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
)

type LogRotate struct {
	srcFile  string
	dstFile  string
	ctx      context.Context
	cancel   context.CancelFunc
	interval time.Duration
	logger   *zap.Logger
	mu       sync.Mutex
}

func NewLogRotate(srcFile string, dstFile string, interval time.Duration, logger *zap.Logger) *LogRotate {
	ctx, cancel := context.WithCancel(context.Background())

	return &LogRotate{
		srcFile:  srcFile,
		dstFile:  dstFile,
		ctx:      ctx,
		cancel:   cancel,
		interval: interval,
		logger:   logger,
	}

}

var ErrStoppedByCancelSignal = fmt.Errorf("stopped by cancel signal")

func (l *LogRotate) Start(rotateChan chan<- bool, processMetricsNotify chan bool) error {

	ticker := time.NewTicker(l.interval)
	defer ticker.Stop()

	for {
		select {
		case <-l.ctx.Done():
			return ErrStoppedByCancelSignal
		case <-ticker.C:
			if err := l.rotate(rotateChan); err != nil {
				return err
			}
			processMetricsNotify <- true

		}
	}
}

func (l *LogRotate) rotate(rotateChan chan<- bool) error {
	// Only lock the mutex when accessing shared state, not during I/O
	l.mu.Lock()
	dstFile := l.dstFile
	srcFile := l.srcFile
	l.mu.Unlock()

	dstDir := filepath.Dir(dstFile)
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return err
	}

	dst, err := os.Create(dstFile)
	if err != nil {
		return err
	}
	defer dst.Close()

	src, err := os.OpenFile(srcFile, os.O_RDWR, 0644)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer src.Close()

	if _, err = io.Copy(dst, src); err != nil {
		return err
	}

	if err = src.Truncate(0); err != nil {
		return err
	}
	for {
		select {
		case rotateChan <- true:
			l.logger.Info(
				"file has been rotated",
				zap.String("redirect_file", srcFile),
				zap.String("rotate_file", dstFile),
				zap.Time("rotated_at", time.Now()),
				zap.String("reason", "scheduled interval"),
			)
			return nil
		case <-l.ctx.Done():
			return ErrStoppedByCancelSignal
		}
	}

}

func (l *LogRotate) Stop() {
	l.cancel()
}
