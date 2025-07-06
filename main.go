package main

import (
	"github.com/akmanon/kpi-metricsd/internal/app"
)

func main() {
	app := app.NewApp()
	app.Run()
}
