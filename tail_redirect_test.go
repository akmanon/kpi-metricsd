package main

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTailAndRedirect_Start_WritesNewLines(t *testing.T) {
	srcF := "app_test.log"
	dstF := "app_test_redirect.log"
	defer os.Remove(srcF)
	defer os.Remove(dstF)

	// Create source file and write initial content
	srcFile, err := os.Create(srcF)
	assert.NoError(t, err)
	srcFile.Close()

	ctx, cancel := context.WithCancel(context.Background())
	tr := NewTailAndRedirect(srcF, dstF, ctx)

	// Run Start in a goroutine
	done := make(chan error)
	go func() {
		done <- tr.Start()
	}()

	// Give Start time to set up watcher and seek to end
	// (in real code, use sync or channel, but for test, sleep is ok)
	// This is needed so the watcher is ready before we write
	// Otherwise, the write event may be missed
	// 100ms is usually enough for local fsnotify
	//nolint:gomnd
	<-trReady(tr)

	// Write a new line to the source file
	timeSleep(500)
	appendLine := "hello world\nhello world\nhello world\nhello world\nhello world\nhello world\nhello world\nhello world\nhello world\nhello world\nhello world\nhello world\nhello world\nhello world\nhello world\nhello world\nhello world\nhello world\n"
	f, err := os.OpenFile(srcF, os.O_APPEND|os.O_WRONLY, 0644)
	assert.NoError(t, err)
	_, err = f.WriteString(appendLine)
	assert.NoError(t, err)
	f.Close()

	// Give some time for the watcher to pick up the event and process
	//nolint:gomnd
	waitForFileContains(dstF, appendLine, t)

	// Cancel the context to stop Start
	cancel()
	<-done

	// Check that the destination file contains the appended line
	dstContent, err := os.ReadFile(dstF)
	assert.NoError(t, err)
	assert.Contains(t, string(dstContent), appendLine)
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
	//nolint:gomnd
	for i := 0; i < 20; i++ {
		b, err := os.ReadFile(filename)
		if err == nil && string(b) == expected {
			return
		}
		//nolint:gomnd
		timeSleep(100)
	}
	t.Fatalf("file %s did not contain expected content after waiting", filename)
}

// timeSleep is a helper to allow easy mocking in tests.
var timeSleep = func(ms int) {
	//nolint:gomnd
	time.Sleep(time.Duration(ms) * time.Millisecond)
}
