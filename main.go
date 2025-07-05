package main

import (
	"context"
)

func main() {
	//f, _ := os.Create("test_log/app.log")
	//f.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	t := NewTailAndRedirect("test_log/app.log", "test_log/app_redirect.log", ctx)
	t.Start()
}
