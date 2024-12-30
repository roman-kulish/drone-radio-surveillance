package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"gtihub.con/roman-kulish/radio-surveillance/cmd/sweeper/app"
	"gtihub.con/roman-kulish/radio-surveillance/internal/sdr"
	"gtihub.con/roman-kulish/radio-surveillance/internal/sdr/hackrf"
	"gtihub.con/roman-kulish/radio-surveillance/internal/sdr/rtl"
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

	var devices []*sdr.Device
	for _, deviceConfig := range config.Devices {
		if !deviceConfig.Enabled {
			continue
		}
		switch deviceConfig.Type {
		case app.DeviceRTLSDR:
			runner, err := rtl.New(deviceConfig.Config.(*rtl.Config))
			if err != nil {
				logger.Error(fmt.Sprintf("failed to create RTL-SDR device: %s", err.Error()))
				os.Exit(1)
			}
			devices = append(devices, sdr.NewDevice(deviceConfig.Name, runner, sdr.WithLogger(logger)))

		case app.DeviceHackRF:
			runner, err := hackrf.New(deviceConfig.Config.(*hackrf.Config))
			if err != nil {
				logger.Error(fmt.Sprintf("failed to create HackRF device: %s", err.Error()))
				os.Exit(1)
			}
			devices = append(devices, sdr.NewDevice(deviceConfig.Name, runner, sdr.WithLogger(logger)))

		default:
			logger.Error(fmt.Sprintf("failed to create device: unknown type '%s'", deviceConfig.Type))
			os.Exit(1)
		}
	}

	// TODO: storage
	// TODO: telemetry
	// TODO: collector
}
