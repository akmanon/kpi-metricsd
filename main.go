package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"time"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	t := NewTailAndRedirect("test_log/app.log", "test_log/app_redirect.log", ctx)
	go func() {
		if err := t.Start(); err != nil {
			os.Stderr.WriteString("Error starting tail: " + err.Error() + "\n")
			return
		}
	}()

	go func() {
		f, err := os.OpenFile("test_log/app.log", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
		if err != nil {
			log.Fatal(err)
		}
		writer := bufio.NewWriter(f)
		for {
			for i := range 10000 {
				writer.WriteString(fmt.Sprintf("%d, this is test log\n", i))
			}
			time.Sleep(500 * time.Millisecond)
		}
	}()

	select {}

}
