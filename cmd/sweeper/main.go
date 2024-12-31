package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.con/roman-kulish/radio-surveillance/cmd/sweeper/app"
)

func main() {
	var logLevel slog.LevelVar
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: &logLevel}))

	var configPath string
	flag.StringVar(&configPath, "c", "", "Path to the configuration file")
	flag.Parse()

	if configPath == "" {
		logger.Error("no configuration file provided")
		os.Exit(1)
	}

	config, err := app.LoadConfig(configPath)
	if err != nil {
		logger.Error(fmt.Sprintf("failed to load configuration file: %s", err.Error()), slog.String("path", configPath))
		os.Exit(1)
	}

	logLevel.Set(config.Settings.LogLevel)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err = app.Run(ctx, config, logger); err != nil {
		logger.Error(err.Error())

		cancel()
		os.Exit(1)
	}
}
