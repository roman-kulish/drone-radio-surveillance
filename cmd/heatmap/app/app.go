package app

import (
	"context"
	"fmt"
	"image/jpeg"
	"image/png"
	"log/slog"
	"os"
	"time"

	"github.com/roman-kulish/radio-surveillance/internal/spectrum"
	"github.com/roman-kulish/radio-surveillance/internal/storage"
)

func Run(ctx context.Context, config *Config, logger *slog.Logger) error {
	if _, err := os.Stat(config.DBPath); err != nil && os.IsNotExist(err) {
		return fmt.Errorf("database file '%s' does not exist: %w", config.DBPath, err)
	}

	store := storage.NewSqliteStore(config.DBPath)
	defer store.Close()

	return readSpectrum(ctx, store, config, logger)
}

func readSpectrum(ctx context.Context, store *storage.SqliteStore, config *Config, logger *slog.Logger) error {
	type T = spectrum.SpectralPoint

	var opts []storage.ReaderOption[T]
	var filters []any
	switch {
	case config.MinFrequency != nil && config.MaxFrequency != nil:
		opts = append(opts, storage.WithFreqRange[T](*config.MinFrequency, *config.MaxFrequency))

		filters = append(filters,
			slog.String("minFreq", fmt.Sprintf("%0.2fHz", *config.MinFrequency)),
			slog.String("maxFreq", fmt.Sprintf("%0.2fHz", *config.MaxFrequency)))

	case config.MinFrequency != nil:
		opts = append(opts, storage.WithMinFreq[T](*config.MinFrequency))
		filters = append(filters, slog.String("minFreq", fmt.Sprintf("%0.2fHz", *config.MinFrequency)))

	case config.MaxFrequency != nil:
		opts = append(opts, storage.WithMaxFreq[T](*config.MaxFrequency))
		filters = append(filters, slog.String("maxFreq", fmt.Sprintf("%0.2fHz", *config.MaxFrequency)))
	}

	switch {
	case config.MinTimestamp != nil && config.MaxTimestamp != nil:
		opts = append(opts, storage.WithTimeRange[T](config.MinTimestamp.UTC(), config.MaxTimestamp.UTC()))

		filters = append(filters,
			slog.String("minTimestamp", config.MinTimestamp.UTC().Format(time.DateTime)),
			slog.String("maxTimestamp", config.MaxTimestamp.UTC().Format(time.DateTime)))

	case config.MinTimestamp != nil:
		opts = append(opts, storage.WithStartTime[T](config.MinTimestamp.UTC()))
		filters = append(filters, slog.String("minTimestamp", config.MinTimestamp.UTC().Format(time.DateTime)))

	case config.MaxTimestamp != nil:
		opts = append(opts, storage.WithEndTime[T](config.MaxTimestamp.UTC()))
		filters = append(filters, slog.String("maxTimestamp", config.MaxTimestamp.UTC().Format(time.DateTime)))
	}

	logger.Info("iterator configuration", filters...)

	iter, err := store.ReadSpectrum(ctx, config.SessionID, opts...)
	if err != nil {
		return err
	}
	defer iter.Close()

	logger.Info("reading data points, hold on tight, it will take a while")

	spec := NewSpectrumData(NewSmoothBounds(0.3))
	for iter.Next(ctx) {
		spec.Update(iter.Current())
	}
	if err = iter.Error(); err != nil {
		return err
	}

	bounds := spec.BoundsTracker.Current()

	logger.Info("finished reading data points",
		slog.Group("stats",
			slog.String("minTimestamp", spec.TimestampStart.Local().Format(time.DateTime)),
			slog.String("maxTimestamp", spec.TimestampEnd.Local().Format(time.DateTime)),
			slog.String("minFreq", fmt.Sprintf("%0.2fHz", spec.FrequencyMin)),
			slog.String("maxFreq", fmt.Sprintf("%0.2fHz", spec.FrequencyMax)),
			slog.String("minPower", fmt.Sprintf("%0.2fdB", bounds.Min)),
			slog.String("maxPower", fmt.Sprintf("%02.fdB", bounds.Max)),
		))

	renderer, err := NewSpectrumRenderer(RenderConfig{
		Location:   config.TimeZone,
		ColorTheme: config.Theme,
	})
	if err != nil {
		return fmt.Errorf("creating spectrum renderer: %w", err)
	}

	logger.Info("rendering spectrum",
		slog.Group("image",
			slog.String("destination", config.OutputFile),
			slog.String("format", string(config.Format)),
			slog.String("theme", string(config.Theme)),
			slog.Int("width", spec.Width),
			slog.Int("height", spec.Height),
		))

	img, err := renderer.Render(spec)
	if err != nil {
		return fmt.Errorf("rendering spectrum: %w", err)
	}

	out, err := os.Create(config.OutputFile)
	if err != nil {
		return err
	}

	switch config.Format {
	case ImagePNG:
		err = png.Encode(out, img)
		break

	case ImageJPEG:
		err = jpeg.Encode(out, img, &jpeg.Options{
			Quality: 98,
		})
		break
	}
	return err
}
