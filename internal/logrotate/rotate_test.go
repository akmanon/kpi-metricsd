package logrotate

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestRotateLog(t *testing.T) {

	t.Run("should return error when stopped by cancel signal", func(t *testing.T) {
		srcFile := "test_log/app_cancel.log"
		destFile := "test_log/app_cancel_rotate.log"
		interval := time.Second * 1

		os.MkdirAll(filepath.Dir(srcFile), 0755)
		f, _ := os.Create(srcFile)
		f.Write([]byte("CancelTest"))
		f.Close()
		rotateChan := make(chan bool)

		logRotate := NewLogRotate(srcFile, destFile, interval, zap.NewNop())

		// Stop the log rotator almost immediately
		time.AfterFunc(time.Millisecond*100, func() {
			logRotate.Stop()
		})

		err := logRotate.Start(rotateChan)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "stopped by cancel signal")
	})

	t.Run("should handle error if source file does not exist", func(t *testing.T) {
		srcFile := "test_log/nonexistent.log"
		destFile := "test_log/nonexistent_rotate.log"
		interval := time.Millisecond * 200
		rotateChan := make(chan bool)

		logRotate := NewLogRotate(srcFile, destFile, interval, zap.NewNop())

		time.AfterFunc(time.Millisecond*400, func() {
			logRotate.Stop()
		})

		err := logRotate.Start(rotateChan)
		assert.Error(t, err)
		assert.True(t, os.IsNotExist(err), "expected file not exist error")
		cleanUpTestDir()
	})

	t.Run("there should be a rotated dest file", func(t *testing.T) {
		var err error

		srcFile := "test_log/app_redirect.log"
		destFile := "test_log/app_redirect_rotate.log"
		interval := time.Millisecond * 500

		os.MkdirAll(filepath.Dir(srcFile), 0755)
		f, _ := os.Create(srcFile)
		f.WriteString("HelloWorld")
		f.Close()
		rotateChan := make(chan bool)

		logRotate := NewLogRotate(srcFile, destFile, interval, zap.NewNop())

		time.AfterFunc(time.Millisecond*700, func() {
			logRotate.Stop()
			cleanUpTestDir()
		})

		err = logRotate.Start(rotateChan)
		assert.Error(t, err)
		dstData, _ := os.ReadFile(destFile)
		assert.Equal(t, "HelloWorld", string(dstData), "got %s, want %s", "HelloWorld", dstData)

	})

}

func TestLogRotate_RotatedFileExistsAndContentMatches(t *testing.T) {
	t.Run("rotated file should exist and match source content", func(t *testing.T) {
		srcFile := "test_log/rotate_file_exists.log"
		dstFile := "test_log/rotate_file_exists_rotated.log"
		interval := time.Millisecond * 120

		os.MkdirAll(filepath.Dir(srcFile), 0755)
		srcContent := []byte("Rotated file content check")
		os.WriteFile(srcFile, srcContent, 0644)
		rotateChan := make(chan bool)

		logRotate := NewLogRotate(srcFile, dstFile, interval, zap.NewNop())
		time.AfterFunc(time.Millisecond*170, func() {
			logRotate.Stop()
			close(rotateChan)
		})

		err := logRotate.Start(rotateChan)
		assert.Error(t, err)
		assert.Equal(t, ErrStoppedByCancelSignal, err)

		// Check rotated file exists
		_, statErr := os.Stat(dstFile)
		assert.NoError(t, statErr, "rotated file should exist")

		// Check content matches
		dstContent, readErr := os.ReadFile(dstFile)
		assert.NoError(t, readErr)
		assert.Equal(t, srcContent, dstContent, "rotated file content should match source")

		// Source file should be truncated
		srcStat, _ := os.Stat(srcFile)
		assert.Equal(t, int64(0), srcStat.Size(), "source file should be truncated")

		cleanUpTestDir()
	})
}

func cleanUpTestDir() {
	println("Removing the folder")
	os.RemoveAll("test_log")
}
