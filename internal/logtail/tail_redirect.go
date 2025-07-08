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
	dstFilename        string
	dstFilePath        string
	dstFile            *os.File
	srcFile            *os.File
	srcFilename        string
	srcFilePath        string
	srcScanner         *bufio.Reader
	srcFileShortName   string
	srcOffset          int64
	srcPollTruncate    time.Duration
	dstWriter          *bufio.Writer
	ctx                context.Context
	fsWatcher          *fsnotify.Watcher
	truncateDetectorCh chan struct{}
	truncateErrCh      chan error
	logger             *zap.Logger
	cancel             context.CancelFunc
}

func NewTailAndRedirect(srcFile string, dstFile string, logger *zap.Logger) *TailAndRedirect {
	ctx, cancel := context.WithCancel(context.Background())
	truncateDetector := make(chan struct{}, 1)
	dstFilePath := filepath.Dir(dstFile)
	srcFilePath := filepath.Dir(srcFile)
	srcFileShortName := filepath.Base(srcFile)
	truncateErrCh := make(chan error, 1)
	return &TailAndRedirect{
		dstFilePath:        dstFilePath,
		dstFilename:        dstFile,
		srcFilename:        srcFile,
		srcFileShortName:   srcFileShortName,
		srcFilePath:        srcFilePath,
		ctx:                ctx,
		truncateDetectorCh: truncateDetector,
		truncateErrCh:      truncateErrCh,
		srcPollTruncate:    time.Millisecond * 200,
		logger:             logger,
		cancel:             cancel,
	}
}

// Start sets up file handles, watchers, and starts processing lines from the source file to the destination. Returns on error.
func (t *TailAndRedirect) Start(rotateChan <-chan bool) error {
	var fileMutex sync.RWMutex

	if err := t.openDstFile(); err != nil {
		t.logger.Error("Unable to open destination file", zap.String("file", t.dstFilename), zap.Error(err))
		return err
	}

	if err := t.initFsWatcher(); err != nil {
		t.logger.Error("fsnotify failed to initialize", zap.Error(err))
		return err
	}

	go func() {
		for range rotateChan {
			if t.dstFile != nil {
				fileMutex.Lock()
				_, err := t.dstFile.Seek(0, io.SeekStart)
				if err != nil {
					t.logger.Error("Unable to seek destination file", zap.String("dst_file", t.dstFilename), zap.Error(err))
				}
				t.logger.Info("received rotation signal seeking to 0", zap.String("dst_file", t.dstFilename))
				fileMutex.Unlock()

			}

		}
	}()

	go func() {
		if err := t.detectTruncate(); err != nil {
			t.truncateErrCh <- err
		}
	}()

	if err := t.initOpenSrcFile(); err != nil {
		t.logger.Error("failed to open source log file", zap.Error(err))
	}

	for {
		t.readLineAndRedirect()
		err := t.watchSrcFileEvent()
		if err != nil {
			if err == ErrFileDeleted {
				if err := t.initOpenSrcFile(); err != nil {
					t.logger.Error("failed to open source log file", zap.Error(err))
				}
			} else {
				return err
			}
		}
	}
}

// openDstFile opens the destination file for writing and sets up a buffered writer.
func (t *TailAndRedirect) openDstFile() error {
	err := os.MkdirAll(t.dstFilePath, 0755)
	if err != nil {
		return fmt.Errorf("failed to create destination path %s: %w", t.dstFilePath, err)
	}
	f, err := os.Create(t.dstFilename)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", t.dstFilename, err)
	}
	t.dstFile = f
	t.dstWriter = bufio.NewWriterSize(f, 64*1024)
	t.logger.Info("destination file open successfully", zap.String("dst_file", t.dstFilename))
	return nil
}

// initFsWatcher initializes a filesystem watcher on the directory containing the source file.
func (t *TailAndRedirect) initFsWatcher() error {
	var err error
	t.fsWatcher, err = fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("error while setting up fsnotify watcher %w", err)
	}
	if err := t.fsWatcher.Add(t.srcFilePath); err != nil {
		return fmt.Errorf("error while adding fsnotify watch on %s: %w", t.srcFilePath, err)
	}
	t.logger.Info("fsnotify watcher initiated", zap.String("src_path", t.srcFilePath))
	return nil
}

// initOpenSrcFile tries to open and seek to the end of the source file, retrying on failure until successful or an error occurs.
func (t *TailAndRedirect) initOpenSrcFile() error {
	for {
		err := t.openSrcFileAndSeekEnd()
		if err == nil {
			break
		}
		if os.IsNotExist(err) {
			t.logger.Info("source log not exist, waiting", zap.String("src_file", t.srcFilename))
		} else {
			t.logger.Info("failed to open source file, retrying...", zap.String("src_file", t.srcFilename))
		}
		err = t.watchSrcFileEvent()
		if err != nil {
			fmt.Println(err)
			if err == ErrFileDeleted {
				fmt.Println(err)
			} else {
				return err
			}
		}
	}
	return nil
}

var ErrFileDeleted = errors.New("fsnotify source log file removed or renamed")

func (t *TailAndRedirect) watchSrcFileEvent() error {
	select {
	case <-t.ctx.Done():
		return fmt.Errorf("fsnotify source event watcher stopped")
	case event, ok := <-t.fsWatcher.Events:
		if !ok {
			return fmt.Errorf("fsnotify source event watcher channel closed")
		}
		eventFile := filepath.Base(event.Name)
		if eventFile != t.srcFileShortName {
			return nil
		}

		if event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) {
			t.logger.Info("fsnotify source log file removed or renamed")
			if t.srcFile != nil {
				t.resetSrcFile()
			}
			return ErrFileDeleted

		} else if event.Has(fsnotify.Create) {
			t.logger.Info("fsnotify source log file created")
			if t.srcFile == nil {
				if err := t.openSrcFileAndSeekEnd(); err != nil {
					t.logger.Info("failed to open source file, retrying...", zap.String("src_file", t.srcFilename))
				}

			}
		}
	case <-t.truncateDetectorCh:
		t.logger.Info("received file truncate signal, reopening source file")
		if err := t.openSrcFileAndSeekEnd(); err != nil {
			t.logger.Info("failed to open source file, retrying...", zap.String("src_file", t.srcFilename))
		}
	case <-t.truncateErrCh:
		return fmt.Errorf("detect truncate stopped")
	}
	return nil
}

// openSrcFileAndSeekEnd opens the source file, seeks to end, and sets up a buffered reader.
func (t *TailAndRedirect) openSrcFileAndSeekEnd() error {
	t.resetSrcFile()
	var err error
	t.srcFile, err = os.OpenFile(t.srcFilename, os.O_RDONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open source file %s: %w", t.srcFilename, err)
	}
	if t.srcOffset, err = t.srcFile.Seek(0, io.SeekEnd); err != nil {
		t.srcFile.Close()
		return fmt.Errorf("failed to seek end of source file %s: %w", t.srcFilename, err)
	}
	t.srcScanner = bufio.NewReaderSize(t.srcFile, 64*1024)
	t.logger.Info("source file opened successfully")
	return nil
}

// resetSrcFile closes the current source file, and resets the srcFile and srcScanner fields to nil.
// This is typically used to clean up resources and prepare for reopening or reinitializing the source file.
func (t *TailAndRedirect) resetSrcFile() {
	if t.srcFile != nil {
		t.srcFile.Close()
		t.srcFile = nil
	}
	t.srcScanner = nil
}

func (t *TailAndRedirect) detectTruncate() error {
	ticker := time.NewTicker(t.srcPollTruncate)
	defer ticker.Stop()
	for {
		select {
		case <-t.ctx.Done():
			return t.ctx.Err()
		case <-ticker.C:
			if t.srcFile != nil {
				info, err := t.srcFile.Stat()
				if err != nil {
					fmt.Printf("error while getting info truncate poller")
					continue
				}
				if info.Size() < t.srcOffset {
					t.truncateDetectorCh <- struct{}{}
				}

			}
		}

	}

}

func (t *TailAndRedirect) readLineAndRedirect() {
	if t.srcScanner == nil {
		t.logger.Warn("srcScanner is nil, skipping readLineAndRedirect")
		return
	}
	for {
		lineBytes, err := t.srcScanner.ReadBytes('\n')
		if len(lineBytes) > 0 {
			if _, werr := t.dstWriter.Write(lineBytes); werr != nil {
				t.logger.Error("failed to write to destination file", zap.Error(werr))
				break
			}
			if ferr := t.dstWriter.Flush(); ferr != nil {
				t.logger.Error("failed to flush to destination file", zap.Error(ferr))
				break
			}
		}
		if err != nil {
			if err == io.EOF && t.srcFile != nil {
				if off, serr := t.srcFile.Seek(0, io.SeekCurrent); serr == nil {
					t.srcOffset = off
				}
			}
			break
		}
	}
}

func (t *TailAndRedirect) Stop() {
	t.cancel()
}
