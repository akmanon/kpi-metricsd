package logrotate

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"time"
)

type LogRotate struct {
	srcFile  string
	dstFile  string
	ctx      context.Context
	cancel   context.CancelFunc
	interval time.Duration
}

func NewLogRotate(srcFile string, dstFile string, interval time.Duration) *LogRotate {
	ctx, cancel := context.WithCancel(context.Background())
	return &LogRotate{
		srcFile:  srcFile,
		dstFile:  dstFile,
		ctx:      ctx,
		cancel:   cancel,
		interval: interval,
	}

}

func (l *LogRotate) Start() error {

	ticker := time.NewTicker(l.interval)

	for {
		select {
		case <-l.ctx.Done():
			return nil
		case <-ticker.C:
			if err := l.rotate(); err != nil {
				return err
			}
		}
	}
}

func (l *LogRotate) rotate() error {
	dstDir := filepath.Dir(l.dstFile)
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return err
	}

	dst, err := os.Create(l.dstFile)
	if err != nil {
		return err
	}
	defer dst.Close()

	src, err := os.OpenFile(l.srcFile, os.O_RDWR, 0644)
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

	return nil
}

func (l *LogRotate) Stop() {
	l.cancel()
}
