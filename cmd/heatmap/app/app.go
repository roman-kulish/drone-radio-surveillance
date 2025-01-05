package app

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"log/slog"
	"os"

	"github.com/roman-kulish/radio-surveillance/internal/spectrum"
	"github.com/roman-kulish/radio-surveillance/internal/storage"
)

type Spectrum struct {
	Width, Height int
	PowerBounds   PowerBounds
	Spans         [][]spectrum.SpectralPoint
}

func Run(ctx context.Context, config *Config, logger *slog.Logger) error {
	if _, err := os.Stat(config.DBPath); err != nil && os.IsNotExist(err) {
		return fmt.Errorf("database file '%s' does not exist: %w", config.DBPath, err)
	}

	store := storage.New(config.DBPath)
	defer store.Close()

	spec, err := readSpectrum(ctx, store, config, logger)
	if err != nil {
		return fmt.Errorf("failed to read session data: %w", err)
	}

	return renderSpectrum(spec, config)
}

func readSpectrum(ctx context.Context, store storage.Store, config *Config, logger *slog.Logger) (*Spectrum, error) {
	iter, err := store.ReadSpectrum(ctx, config.SessionID)
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	logger.Info("reading data points ...")

	var bounds PowerBounds
	boundsTracker := NewSmoothBounds(0.3)

	var width, height int
	var spans [][]spectrum.SpectralPoint
	for iter.Next(ctx) {
		span := iter.Current()

		n := len(span.Samples)
		if width < n {
			width = n
		}
		height++

		powers := make([]float64, len(span.Samples))
		for i, sample := range span.Samples {
			if sample.Power == nil {
				continue
			}
			powers[i] = *sample.Power
		}

		bounds = boundsTracker.Update(powers)
		spans = append(spans, span.Samples)
	}
	if err = iter.Error(); err != nil {
		return nil, err
	}

	s := Spectrum{
		PowerBounds: bounds,
		Width:       width,
		Height:      height,
		Spans:       spans,
	}

	logger.Info("finished reading data points",
		slog.Int("width", s.Width),
		slog.Int("height", s.Height),
		slog.Float64("minPower", s.PowerBounds.Min),
		slog.Float64("maxPower", s.PowerBounds.Max))

	return &s, nil
}

func renderSpectrum(spec *Spectrum, config *Config) error {
	colorMap := CreateColorMap(256, true) // true for enhanced color mapping
	img := image.NewRGBA(image.Rect(0, 0, spec.Width, spec.Height))
	for y, r := range spec.Spans {
		for x, s := range r {
			if s.Power == nil {
				img.Set(x, y, color.Black)
				continue
			}

			normalized := NormalizePower(*s.Power, spec.PowerBounds.Min, spec.PowerBounds.Max)

			// Map to color map index
			colorIdx := int(normalized * 255)
			if colorIdx < 0 {
				colorIdx = 0
			} else if colorIdx > 255 {
				colorIdx = 255
			}

			img.Set(x, y, colorMap[colorIdx])
		}
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
