package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/roman-kulish/radio-surveillance/internal/storage"
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

	var opts []OrchestratorOption

	// TODO: telemetry

	orchestrator := NewOrchestrator(store, logger, opts...)
	for _, c := range config.Devices {
		if err = orchestrator.CreateDevice(&c); err != nil {
			return fmt.Errorf("failed to create device: %w", err)
		}
	}

	return orchestrator.Run(ctx)
}

func createStorage(config *StorageConfig) (storage.Store, error) {
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
	return storage.New(dbPath), nil
}
