package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
)

type TailAndRedirect struct {
	srcFile string
	dstFile string
	ctx     context.Context
}

func NewTailAndRedirect(srcFile string, dstFile string, ctx context.Context) *TailAndRedirect {
	return &TailAndRedirect{srcFile: srcFile, dstFile: dstFile, ctx: ctx}
}

func (t *TailAndRedirect) Start() error {
	path := filepath.Dir(t.srcFile)
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("watching %s path for events", path)
	watcher.Add(path)
	srcFile, err := os.OpenFile(t.srcFile, os.O_RDONLY, 0644)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(t.dstFile)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	srcFile.Seek(0, io.SeekEnd)
	//reader := bufio.NewReader(srcFile)
	reader := bufio.NewReaderSize(srcFile, 64*1024)
	writer := bufio.NewWriterSize(dstFile, 64*1024)
	//reader := bufio.NewReaderSize()

	fmt.Println("Start Processing")
	for {
		select {
		case <-t.ctx.Done():
			fmt.Println("taling stopped by context cancelation")
			return nil
		case event, ok := <-watcher.Events:
			if !ok {
				log.Fatal("Watcher channel closed")
			}
			if event.Op&fsnotify.Write == fsnotify.Write {
				line, err := reader.ReadBytes('\n')
				if err != nil {
					if err == io.EOF {
						continue
					}
					fmt.Println("Read error:", err)
					break
				}
				_, err = writer.Write(line)
				if err != nil {
					fmt.Println("Write error:", err)
					break
				}
				writer.Flush()
			}

		}
	}
}
