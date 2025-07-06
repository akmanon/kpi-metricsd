package logrotate

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRotateLog(t *testing.T) {

	t.Run("expected err if no source redirect file exist", func(t *testing.T) {
		var err error
		srcFile := "test_log/app_redirect.log"
		destFile := "test_log/app_redirect_rotate.log"
		interval := time.Second * 2

		logRotate := NewLogRotate(srcFile, destFile, interval)
		time.AfterFunc((interval + 1), logRotate.Stop)
		err = logRotate.Start()
		assert.Error(t, err)
		if err != nil {
			if !os.IsNotExist(err) {
				t.Errorf("unexpected error while stating file")
			}
		}
	})
	t.Run("there should be a rotated dest file", func(t *testing.T) {
		var err error

		srcFile := "test_log/app_redirect.log"
		destFile := "test_log/app_redirect_rotate.log"
		interval := time.Second * 2

		os.MkdirAll(filepath.Dir(srcFile), 0755)
		f, _ := os.Create(srcFile)
		srcData := []byte("HelloWorld")
		f.Write(srcData)
		f.Close()

		logRotate := NewLogRotate(srcFile, destFile, interval)

		time.AfterFunc((interval + 1), logRotate.Stop)

		err = logRotate.Start()
		assert.NoError(t, err)
		if err != nil {
			if !os.IsNotExist(err) {
				t.Errorf("unexpected error while stating file")
			}
		}
		dstData, _ := os.ReadFile(destFile)
		assert.Equal(t, srcData, dstData, "got %s, want %s", srcData, dstData)
		cleanUpTestDir()
	})

}
func cleanUpTestDir() {
	os.RemoveAll("test_log")
}
