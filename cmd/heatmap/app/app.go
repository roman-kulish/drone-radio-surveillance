package app

import (
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"log/slog"
	"os"

	"github.com/roman-kulish/radio-surveillance/internal/storage"
)

func Run(ctx context.Context, config *Config, logger *slog.Logger) error {
	if _, err := os.Stat(config.DBPath); err != nil && os.IsNotExist(err) {
		return fmt.Errorf("database file '%s' does not exist: %w", config.DBPath, err)
	}

	store := storage.NewSqliteStore(config.DBPath)
	defer store.Close()

	spec, err := readSpectrum(ctx, store, config, logger)
	if err != nil {
		return fmt.Errorf("failed to read session data: %w", err)
	}

	return renderSpectrum(spec, config)
}

func readSpectrum(ctx context.Context, store *storage.SqliteStore, config *Config, logger *slog.Logger) (*SpectrumData, error) {
	iter, err := store.ReadSpectrum(ctx, config.SessionID)
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	logger.Info("reading data points ...")

	spec := NewSpectrumData(NewSmoothBounds(0.3))
	for iter.Next(ctx) {
		spec.Update(iter.Current())
	}
	if err = iter.Error(); err != nil {
		return nil, err
	}

	bounds := spec.BoundsTracker.Current()

	logger.Info("finished reading data points",
		slog.Int("image width", spec.Width),
		slog.Int("image height", spec.Height),
		slog.Float64("minFreq", spec.FrequencyMin),
		slog.Float64("maxFreq", spec.FrequencyMax),
		slog.Float64("minPower", bounds.Min),
		slog.Float64("maxPower", bounds.Max))

	return spec, nil
}

func renderSpectrum(spec *SpectrumData, config *Config) error {
	colorMap := NewColorMapper(config.Theme, spec.BoundsTracker.Current())
	img := image.NewRGBA(image.Rect(0, 0, spec.Width, spec.Height))
	for y, span := range spec.Spans {
		for x, power := range span {
			img.Set(x, y, colorMap.GetColor(power))
		}
	}

	ann, err := NewAnnotator()
	if err != nil {
		return err
	}
	if err = ann.Annotate(img, spec); err != nil {
		return err
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
