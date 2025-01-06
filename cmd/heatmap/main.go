package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/roman-kulish/radio-surveillance/cmd/heatmap/app"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config, err := app.NewConfigFromCLI()
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err = app.Run(ctx, config, logger); err != nil {
		logger.Error(err.Error())

		cancel()
		os.Exit(1)
	}
}
