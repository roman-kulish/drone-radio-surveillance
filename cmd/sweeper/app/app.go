package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"gtihub.con/roman-kulish/radio-surveillance/internal/sdr"
	"gtihub.con/roman-kulish/radio-surveillance/internal/sdr/hackrf"
	"gtihub.con/roman-kulish/radio-surveillance/internal/sdr/rtl"
	"gtihub.con/roman-kulish/radio-surveillance/internal/storage"
)

const (
	storageDir = "data"
)

func Run(ctx context.Context, config *Config, logger *slog.Logger) error {
	store, err := createStorage(&config.Storage)
	if err != nil {
		return fmt.Errorf("failed to create storage: %w", err)
	}
	defer store.Close()

	devices, err := createDevices(config.Devices, store, logger)
	if err != nil {
		return fmt.Errorf("failed to create devices: %w", err)
	}
	if len(devices) == 0 {
		return fmt.Errorf("no devices specified on configuration")
	}

	// TODO: telemetry
	// TODO: collector

	return nil
}

func createDevices(config []DeviceConfig, store *storage.Store, logger *slog.Logger) ([]*sdr.Device, error) {
	var devices []*sdr.Device
	for _, deviceConfig := range config {
		if !deviceConfig.Enabled {
			continue
		}

		var handler sdr.Handler
		var err error
		switch deviceConfig.Type {
		case DeviceRTLSDR:
			if handler, err = rtl.New(deviceConfig.Config.(*rtl.Config)); err != nil {
				return nil, fmt.Errorf("creating RTL-SDR device: %w", err)
			}

		case DeviceHackRF:
			if handler, err = hackrf.New(deviceConfig.Config.(*hackrf.Config)); err != nil {
				return nil, fmt.Errorf("creating HackRF device: %w", err)
			}

		default:
			return nil, fmt.Errorf("creating device: unknown type '%s'", deviceConfig.Type)
		}

		if _, err = store.CreateSession(string(deviceConfig.Type), deviceConfig.Name, deviceConfig.Config); err != nil {
			return nil, fmt.Errorf("creating session: %w", err)
		}

		devices = append(devices, sdr.NewDevice(deviceConfig.Name, handler, sdr.WithLogger(logger)))
	}

	return devices, nil
}

func createStorage(config *StorageConfig) (*storage.Store, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current working directory: %w", err)
	}

	var dbPath string
	if config.DataDirectory != "" {
		dbPath = filepath.Join(wd, config.DataDirectory)
	} else {
		dbPath = filepath.Join(wd, storageDir)
	}

	stat, err := os.Stat(dbPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("storage directory '%s' does not exist: %w", dbPath, err)
		}
		if !stat.IsDir() {
			return nil, fmt.Errorf("invalid storage directory '%s'", dbPath)
		}
	}

	dbPath = filepath.Join(dbPath, fmt.Sprintf("sdr_session_%s.sqlite", time.Now().UTC().Format("20060102_150405")))
	store, err := storage.New(dbPath)
	if err != nil {
		return nil, fmt.Errorf("creating storage: %w", err)
	}

	return store, nil
}
