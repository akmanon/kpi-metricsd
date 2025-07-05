package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

type TailAndRedirect struct {
	srcFilename        string
	dstFilename        string
	srcFile            *os.File
	srcScanner         *bufio.Reader
	srcOffset          int64
	srcPollTruncate    time.Duration
	dstWriter          *bufio.Writer
	ctx                context.Context
	fsWatcher          *fsnotify.Watcher
	chTruncateDetector chan struct{}
}

func NewTailAndRedirect(srcFile string, dstFile string, ctx context.Context) *TailAndRedirect {
	truncateDetector := make(chan struct{}, 1)
	return &TailAndRedirect{srcFilename: srcFile,
		dstFilename:        dstFile,
		ctx:                ctx,
		chTruncateDetector: truncateDetector,
		srcPollTruncate:    time.Millisecond * 200,
	}
}

func (t *TailAndRedirect) Start() error {
	if err := t.initFsWatcher(); err != nil {
		log.Fatal(err)
	}

	if err := t.openDstFile(); err != nil {
		log.Fatal(err)
	}

	t.initOpenSrcFile()
	go t.detectTruncate()
	fmt.Println("Start Processing")
	for {
		t.readLineAndRedirect()
		if err := t.watchSrcFileEvent(); err != nil {
			return nil
		}
	}
}

func (t *TailAndRedirect) initFsWatcher() error {
	path := filepath.Dir(t.srcFilename)
	var err error
	t.fsWatcher, err = fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	fmt.Printf("watching %s path for events", path)
	t.fsWatcher.Add(path)
	return nil
}

func (t *TailAndRedirect) openSrcFileAndSeekEnd() error {
	if t.srcFile != nil {
		t.srcFile.Close()
		t.srcScanner = nil
	}
	var err error
	t.srcFile, err = os.OpenFile(t.srcFilename, os.O_RDONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", t.srcFilename, err)
	}
	if t.srcOffset, err = t.srcFile.Seek(0, io.SeekEnd); err != nil {
		t.srcFile.Close()
		return fmt.Errorf("failed to seek end of file %s: %w", t.srcFilename, err)
	}
	t.srcScanner = bufio.NewReaderSize(t.srcFile, 64*1024)
	fmt.Println("Successfuly open srcfile")
	return nil
}

func (t *TailAndRedirect) initOpenSrcFile() {
	for {
		err := t.openSrcFileAndSeekEnd()
		if err == nil {
			break
		}
		if os.IsNotExist(err) {
			fmt.Println("File does not exist now, wating...")
		} else {
			fmt.Println("Initial file open error, retrying")
		}
		t.watchSrcFileEvent()
	}
}

func (t *TailAndRedirect) watchSrcFileEvent() error {
	select {
	case <-t.ctx.Done():
		fmt.Println("taling stopped by context cancelation")
		return fmt.Errorf("taling stopped by context cancelation")
	case event, ok := <-t.fsWatcher.Events:
		if !ok {
			log.Fatal("Watcher channel closed")
		}
		eventFile := filepath.Base(event.Name)
		if eventFile != filepath.Base(t.srcFilename) {
			return nil
		}
		if event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) {
			fmt.Println("Some file has been removed")
			if t.srcFile != nil {
				t.srcFile.Close()
				t.srcFile = nil
				t.srcScanner = nil
			}
			t.initOpenSrcFile()
		} else if event.Has(fsnotify.Create) {
			fmt.Println("some file was created")
			if t.srcFile == nil {
				if err := t.openSrcFileAndSeekEnd(); err != nil {
					fmt.Println("Some error while opening file")
				}

			}
		}
	case <-t.chTruncateDetector:
		fmt.Println("received src file truncate signal")
		if err := t.openSrcFileAndSeekEnd(); err != nil {
			fmt.Println("Some error while opening file")
		}

	}
	return nil
}

func (t *TailAndRedirect) detectTruncate() {
	ticker := time.NewTicker(t.srcPollTruncate)
	for {
		select {
		case <-t.ctx.Done():
			fmt.Println("stopping detectTruncate context cancel")
			return
		case <-ticker.C:
			if t.srcFile != nil {
				info, err := t.srcFile.Stat()
				if err != nil {
					fmt.Printf("error while getting info truncate poller")
					continue
				}
				if info.Size() < t.srcOffset {
					t.chTruncateDetector <- struct{}{}
				}

			}
		}

	}

}

func (t *TailAndRedirect) openDstFile() error {
	err := os.MkdirAll(filepath.Dir(t.dstFilename), 0755)
	if err != nil {
		fmt.Println("error while creating dst path")
	}
	fmt.Printf("Dir of Dst Filename %s", filepath.Dir(t.dstFilename))
	f, err := os.Create(t.dstFilename)
	if err != nil {
		fmt.Println("Error whille opening the file")
		return err
	}

	t.dstWriter = bufio.NewWriterSize(f, 64*1024)
	return nil
}

func (t *TailAndRedirect) readLineAndRedirect() {
	fmt.Println("starting the scan")
	if t.srcScanner == nil {
		return
	}
	for {
		lineBytes, err := t.srcScanner.ReadBytes('\n')
		t.dstWriter.Write(lineBytes)
		t.dstWriter.Flush()
		if err != nil {
			if err == io.EOF {
				t.srcOffset, _ = t.srcFile.Seek(0, io.SeekCurrent)
				fmt.Printf("src FIle EOF, %d", t.srcOffset)
			}
			break
		}
	}
}
