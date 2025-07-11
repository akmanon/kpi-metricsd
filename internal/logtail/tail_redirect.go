package logtail

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"go.uber.org/zap"
)

type TailAndRedirect struct {
	dstFilename      string
	dstFilePath      string
	dstFile          *os.File
	srcFile          *os.File
	srcFilename      string
	srcFilePath      string
	srcReader        *bufio.Reader
	srcFileShortName string
	srcOffset        int64

	ctx    context.Context
	cancel context.CancelFunc
	logger *zap.Logger
	mu     sync.RWMutex

	srcPollTruncate    time.Duration
	fsWatcher          *fsnotify.Watcher
	truncateDetectorCh chan struct{}
	truncateErrCh      chan error
	dstWriter          *bufio.Writer
	flushTicker        *time.Ticker
}

func NewTailAndRedirect(srcFile, dstFile string, logger *zap.Logger) *TailAndRedirect {
	ctx, cancel := context.WithCancel(context.Background())
	return &TailAndRedirect{
		dstFilename:        dstFile,
		dstFilePath:        filepath.Dir(dstFile),
		srcFilename:        srcFile,
		srcFilePath:        filepath.Dir(srcFile),
		srcFileShortName:   filepath.Base(srcFile),
		ctx:                ctx,
		cancel:             cancel,
		logger:             logger,
		srcPollTruncate:    200 * time.Millisecond,
		truncateDetectorCh: make(chan struct{}, 1),
		truncateErrCh:      make(chan error, 1),
	}
}

func (t *TailAndRedirect) Start(rotateChan <-chan bool) error {
	if err := t.openDstFile(); err != nil {
		t.logger.Error("failed to open destination", zap.Error(err))
		return err
	}
	if err := t.initFsWatcher(); err != nil {
		t.logger.Error("failed to init fsnotify", zap.Error(err))
		return err
	}
	if err := t.initOpenSrcFile(); err != nil {
		t.logger.Error("failed to init source", zap.Error(err))
	}

	t.flushTicker = time.NewTicker(100 * time.Millisecond)

	go t.handleRotate(rotateChan)
	go t.detectTruncate()
	go t.periodicFlush()

	for {
		select {
		case <-t.ctx.Done():
			return nil
		default:
			t.readLineAndRedirect()
			if err := t.watchSrcFileEvent(); err != nil {
				if err == ErrFileDeleted {
					t.logger.Info("reopening after delete")
					_ = t.initOpenSrcFile()
					continue
				}
				return err
			}
		}
	}
}

func (t *TailAndRedirect) handleRotate(rotateChan <-chan bool) {
	for range rotateChan {
		t.mu.Lock()
		if t.dstFile != nil {
			_, err := t.dstFile.Seek(0, io.SeekStart)
			if err != nil {
				t.logger.Error("seek failed", zap.Error(err))
			}
		}
		t.mu.Unlock()
	}
}

func (t *TailAndRedirect) openDstFile() error {
	if err := os.MkdirAll(t.dstFilePath, 0755); err != nil {
		return fmt.Errorf("mkdir failed: %w", err)
	}
	f, err := os.Create(t.dstFilename)
	if err != nil {
		return fmt.Errorf("create file failed: %w", err)
	}
	t.dstFile = f
	t.dstWriter = bufio.NewWriterSize(f, 64*1024)
	return nil
}

func (t *TailAndRedirect) initOpenSrcFile() error {
	for {
		err := t.openSrcFileAndSeekEnd()
		if err == nil {
			return nil
		}
		if os.IsNotExist(err) {
			t.logger.Info("waiting for source log", zap.String("src", t.srcFilename))
		}
		if err := t.watchSrcFileEvent(); err != nil {
			return err
		}
	}
}

func (t *TailAndRedirect) openSrcFileAndSeekEnd() error {
	t.resetSrcFile()
	f, err := os.OpenFile(t.srcFilename, os.O_RDONLY, 0644)
	if err != nil {
		return err
	}
	t.srcFile = f
	t.srcOffset, err = f.Seek(0, io.SeekEnd)
	if err != nil {
		f.Close()
		return err
	}
	t.srcReader = bufio.NewReaderSize(f, 64*1024)
	t.logger.Info("source file opened", zap.String("file", t.srcFilename))
	return nil
}

func (t *TailAndRedirect) resetSrcFile() {
	if t.srcFile != nil {
		t.srcFile.Close()
	}
	t.srcReader = nil
	t.srcFile = nil
}

func (t *TailAndRedirect) readLineAndRedirect() {

	if t.srcReader == nil {
		return
	}
	for {
		line, err := t.srcReader.ReadBytes('\n')
		if len(line) > 0 {
			t.mu.RLock()
			_, werr := t.dstWriter.Write(line)
			t.mu.RUnlock()
			if werr != nil {
				t.logger.Error("write failed", zap.Error(werr))
				return
			}
		}
		if err != nil {
			if err == io.EOF {
				if t.srcFile != nil {
					if off, serr := t.srcFile.Seek(0, io.SeekCurrent); serr == nil {
						t.srcOffset = off
					}
				}
			}
			break
		}
	}
}

var ErrFileDeleted = errors.New("source file removed or renamed")

func (t *TailAndRedirect) watchSrcFileEvent() error {
	select {
	case <-t.ctx.Done():
		return fmt.Errorf("watcher stopped")
	case event, ok := <-t.fsWatcher.Events:
		if !ok {
			return fmt.Errorf("event channel closed")
		}
		if filepath.Base(event.Name) != t.srcFileShortName {
			return nil
		}
		switch {
		case event.Has(fsnotify.Remove), event.Has(fsnotify.Rename):
			t.logger.Info("file deleted/renamed")
			t.resetSrcFile()
			return ErrFileDeleted
		case event.Has(fsnotify.Create):
			t.logger.Info("file created")
			return t.openSrcFileAndSeekEnd()
		}
	case err := <-t.fsWatcher.Errors:
		return fmt.Errorf("watcher error: %w", err)
	case <-t.truncateDetectorCh:
		t.logger.Info("truncate detected, reopening")
		return t.openSrcFileAndSeekEnd()
	case <-t.truncateErrCh:
		return fmt.Errorf("truncate detector stopped")
	}
	return nil
}

func (t *TailAndRedirect) detectTruncate() {
	ticker := time.NewTicker(t.srcPollTruncate)
	defer ticker.Stop()
	for {
		select {
		case <-t.ctx.Done():
			return
		case <-ticker.C:
			if t.srcFile != nil {
				info, err := t.srcFile.Stat()
				if err == nil && info.Size() < t.srcOffset {
					select {
					case t.truncateDetectorCh <- struct{}{}:
					default:
					}
				}
			}
		}
	}
}

func (t *TailAndRedirect) periodicFlush() {
	for {
		select {
		case <-t.ctx.Done():
			return
		case <-t.flushTicker.C:
			t.mu.Lock()
			if t.dstWriter != nil {
				if err := t.dstWriter.Flush(); err != nil {
					t.logger.Warn("flush failed", zap.Error(err))
				}
			}
			t.mu.Unlock()
		}
	}
}

func (t *TailAndRedirect) initFsWatcher() error {
	var err error
	t.fsWatcher, err = fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	return t.fsWatcher.Add(t.srcFilePath)
}

func (t *TailAndRedirect) Stop() {
	t.cancel()
	t.flushTicker.Stop()
	if t.dstFile != nil {
		t.dstWriter.Flush()
		t.dstFile.Close()
	}
	if t.srcFile != nil {
		t.srcFile.Close()
	}
	if t.fsWatcher != nil {
		t.fsWatcher.Close()
	}
}
