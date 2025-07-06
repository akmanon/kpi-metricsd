package logtail

import (
	"bufio"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestTailAndRedirect(t *testing.T) {
	loggerCfg := zap.NewProductionConfig()
	loggerCfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	logger, _ := loggerCfg.Build()
	defer logger.Sync()

	t.Run("Start_WritesNewLines", func(t *testing.T) {
		srcF := "test_data/app_test.log"
		dstF := "test_data/app_test_redirect.log"
		defer os.RemoveAll(srcF)
		defer os.RemoveAll(filepath.Dir(dstF))

		err := os.MkdirAll(filepath.Dir(dstF), 0755)
		assert.NoError(t, err)
		srcFile, err := os.Create(srcF)
		assert.NoError(t, err)
		srcFile.Close()
		tr := NewTailAndRedirect(srcF, dstF, logger)

		done := make(chan error)
		go func() {
			done <- tr.Start()
		}()

		<-trReady(tr)

		timeSleep(500)
		appendLine := "hello world\n"
		f, err := os.OpenFile(srcF, os.O_APPEND|os.O_WRONLY, 0644)
		assert.NoError(t, err)
		_, err = f.WriteString(appendLine)
		assert.NoError(t, err)
		f.Close()

		waitForFileContains(dstF, appendLine, t)

		tr.Stop()
		<-done

		dstContent, err := os.ReadFile(dstF)
		assert.NoError(t, err)
		assert.Contains(t, string(dstContent), appendLine)
	})

	t.Run("initFsWatcher_Success", func(t *testing.T) {
		tmpDir := t.TempDir()
		srcFile := filepath.Join(tmpDir, "src.log")
		dstFile := filepath.Join(tmpDir, "dst.log")

		tr := NewTailAndRedirect(srcFile, dstFile, logger)
		err := tr.initFsWatcher()
		assert.NoError(t, err)
		assert.NotNil(t, tr.fsWatcher)
		assert.NoError(t, tr.fsWatcher.Close())
	})

	t.Run("initOpenSrcFile_Success", func(t *testing.T) {
		tmpDir := t.TempDir()
		srcFile := filepath.Join(tmpDir, "src.log")
		dstFile := filepath.Join(tmpDir, "dst.log")

		err := os.WriteFile(srcFile, []byte("line1\n"), 0644)
		assert.NoError(t, err)

		tr := NewTailAndRedirect(srcFile, dstFile, logger)
		tr.Stop()
		err = tr.initFsWatcher()
		assert.NoError(t, err)

		err = tr.initOpenSrcFile()
		assert.NoError(t, err)
		assert.NotNil(t, tr.srcFile)
		assert.NotNil(t, tr.srcScanner)
	})

	t.Run("initOpenSrcFile_FileNotExistThenCreated", func(t *testing.T) {
		tmpDir := t.TempDir()
		srcFile := filepath.Join(tmpDir, "src.log")
		dstFile := filepath.Join(tmpDir, "dst.log")
		tr := NewTailAndRedirect(srcFile, dstFile, logger)
		err := tr.initFsWatcher()
		defer tr.Stop()
		assert.NoError(t, err)

		done := make(chan error)
		go func() {
			done <- tr.initOpenSrcFile()
		}()

		timeSleep(200)

		err = os.WriteFile(srcFile, []byte("line2\n"), 0644)
		assert.NoError(t, err)

		tr.fsWatcher.Events <- fsnotify.Event{
			Name: srcFile,
			Op:   fsnotify.Create,
		}

		select {
		case err := <-done:
			assert.NoError(t, err)
			assert.NotNil(t, tr.srcFile)
			assert.NotNil(t, tr.srcScanner)
		case <-time.After(2 * time.Second):
			t.Fatal("initOpenSrcFile did not return after file was created")
		}
	})

	t.Run("initOpenSrcFile_ErrorOnWatchEvent", func(t *testing.T) {
		tmpDir := t.TempDir()
		srcFile := filepath.Join(tmpDir, "src.log")
		dstFile := filepath.Join(tmpDir, "dst.log")

		tr := NewTailAndRedirect(srcFile, dstFile, logger)
		defer tr.Stop()
		err := tr.initFsWatcher()
		assert.NoError(t, err)

		tr.fsWatcher.Close()

		err = tr.initOpenSrcFile()
		assert.Error(t, err)
	})

	t.Run("readLineAndRedirect_WritesLines", func(t *testing.T) {
		tmpDir := t.TempDir()
		srcFile := filepath.Join(tmpDir, "src.log")
		dstFile := filepath.Join(tmpDir, "dst.log")

		lines := []string{"line1\n", "line2\n", "line3\n"}
		err := os.WriteFile(srcFile, []byte(lines[0]+lines[1]+lines[2]), 0644)
		assert.NoError(t, err)

		srcF, err := os.Open(srcFile)
		assert.NoError(t, err)
		defer srcF.Close()

		dstF, err := os.Create(dstFile)
		assert.NoError(t, err)
		defer dstF.Close()

		tr := &TailAndRedirect{
			srcFile:    srcF,
			srcScanner: bufio.NewReaderSize(srcF, 64*1024),
			dstFile:    dstF,
			dstWriter:  bufio.NewWriterSize(dstF, 64*1024),
			logger:     logger,
		}

		tr.readLineAndRedirect()

		dstF.Sync()
		b, err := os.ReadFile(dstFile)
		assert.NoError(t, err)
		assert.Equal(t, lines[0]+lines[1]+lines[2], string(b))
	})

	t.Run("readLineAndRedirect_NoSrcScanner", func(t *testing.T) {
		tr := &TailAndRedirect{
			srcScanner: nil,
			logger:     logger,
		}
		tr.readLineAndRedirect()
	})

	t.Run("readLineAndRedirect_WriteError", func(t *testing.T) {
		r, w := io.Pipe()
		defer r.Close()
		defer w.Close()

		dstFile := &errWriter{}
		tr := &TailAndRedirect{
			srcFile:    nil,
			srcScanner: bufio.NewReaderSize(r, 64*1024),
			dstWriter:  bufio.NewWriterSize(dstFile, 64*1024),
			logger:     logger,
		}

		go func() {
			w.Write([]byte("fail this line\n"))
			w.Close()
		}()

		tr.readLineAndRedirect()
	})

	t.Run("detectTruncate_SendsSignalOnTruncate", func(t *testing.T) {
		tmpDir := t.TempDir()
		srcFile := filepath.Join(tmpDir, "src.log")

		err := os.WriteFile(srcFile, []byte("line1\nline2\n"), 0644)
		assert.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		f, err := os.OpenFile(srcFile, os.O_RDWR, 0644)
		assert.NoError(t, err)
		defer f.Close()

		tr := &TailAndRedirect{
			srcFile:            f,
			srcOffset:          12,
			srcPollTruncate:    10 * time.Millisecond,
			truncateDetectorCh: make(chan struct{}, 1),
			ctx:                ctx,
			logger:             logger,
		}

		done := make(chan error)
		go func() {
			done <- tr.detectTruncate()
		}()

		timeSleep(50)
		err = f.Truncate(5)
		assert.NoError(t, err)

		select {
		case <-tr.truncateDetectorCh:
		case <-time.After(500 * time.Millisecond):
			t.Fatal("did not receive truncate signal")
		}

		cancel()
		<-done
	})

	t.Run("detectTruncate_StopsOnContextDone", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		tr := &TailAndRedirect{
			srcPollTruncate:    10 * time.Millisecond,
			truncateDetectorCh: make(chan struct{}, 1),
			ctx:                ctx,
			logger:             logger,
		}

		done := make(chan error)
		go func() {
			done <- tr.detectTruncate()
		}()

		cancel()
		select {
		case err := <-done:
			assert.Error(t, err)
		case <-time.After(500 * time.Millisecond):
			t.Fatal("detectTruncate did not return after context cancel")
		}
	})

	t.Run("detectTruncate_HandlesStatError", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		tmpDir := t.TempDir()
		srcFile := filepath.Join(tmpDir, "src.log")
		f, err := os.Create(srcFile)
		assert.NoError(t, err)
		f.Close()

		tr := &TailAndRedirect{
			srcFile:            f,
			srcOffset:          0,
			srcPollTruncate:    10 * time.Millisecond,
			truncateDetectorCh: make(chan struct{}, 1),
			ctx:                ctx,
			logger:             logger,
		}

		done := make(chan error)
		go func() {
			done <- tr.detectTruncate()
		}()

		timeSleep(50)
		cancel()
		<-done
	})
}

// errWriter implements io.Writer but always returns an error
type errWriter struct{}

func (e *errWriter) Write(p []byte) (int, error) {
	return 0, os.ErrInvalid
}

// trReady waits until the fsWatcher is initialized and Start is ready to process events.
func trReady(tr *TailAndRedirect) <-chan struct{} {
	ch := make(chan struct{})
	go func() {
		for {
			if tr.fsWatcher != nil {
				close(ch)
				return
			}
		}
	}()
	return ch
}

// waitForFileContains waits until the file contains the expected string or fails the test after timeout.
func waitForFileContains(filename, expected string, t *testing.T) {
	for i := 0; i < 20; i++ {
		b, err := os.ReadFile(filename)
		if err == nil && string(b) == expected {
			return
		}
		timeSleep(100)
	}
	t.Fatalf("file %s did not contain expected content after waiting", filename)
}

// timeSleep is a helper to allow easy mocking in tests.
var timeSleep = func(ms int) {
	time.Sleep(time.Duration(ms) * time.Millisecond)
}
