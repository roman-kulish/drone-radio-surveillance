package app

import (
	"errors"
	"flag"
	"fmt"
	"strings"
	"time"
)

// ImageFormat represents supported output image formats
type ImageFormat string

// Supported image formats
const (
	ImagePNG  ImageFormat = "png"
	ImageJPEG ImageFormat = "jpeg"
)

// Config holds application configuration
type Config struct {
	// File paths
	DBPath     string
	OutputFile string

	// Data selection
	SessionID    int64
	MinFrequency *float64       // Optional frequency filter
	MaxFrequency *float64       // Optional frequency filter
	MinTimestamp *time.Time     // Optional time range filter
	MaxTimestamp *time.Time     // Optional time range filter
	TimeZone     *time.Location // Timezone for time display

	// Visualization
	Theme  ColorTheme
	Format ImageFormat
}

var (
	// validImageFormats defines supported output formats
	validImageFormats = map[ImageFormat]struct{}{
		ImagePNG:  {},
		ImageJPEG: {},
	}

	// validThemes defines supported color themes
	validThemes = map[ColorTheme]struct{}{
		ColorTheme(""): {},
		ClassicTheme:   {},
		GrayscaleTheme: {},
		JungleTheme:    {},
		ThermalTheme:   {},
		MarineTheme:    {},
	}

	// ErrInvalidConfig indicates configuration validation errors
	ErrInvalidConfig = errors.New("invalid configuration")
)

// NewConfig creates a new Config with default values
func NewConfig() *Config {
	return &Config{
		Format:   ImagePNG,
		TimeZone: time.Local,
	}
}

// timeZoneFlag implements flag.Value interface for time.Location
type timeZoneFlag struct {
	location **time.Location
}

func (t *timeZoneFlag) String() string {
	if t.location == nil {
		return "Local"
	}
	return (*t.location).String()
}

func (t *timeZoneFlag) Set(value string) error {
	loc, err := time.LoadLocation(value)
	if err != nil {
		return fmt.Errorf("invalid timezone: %w", err)
	}
	*t.location = loc
	return nil
}

// NewConfigFromCLI creates a Config from command line arguments
func NewConfigFromCLI() (*Config, error) {
	c := NewConfig()

	// Parse basic flags
	var (
		imageFormat string
		theme       string
		minFreq     float64
		maxFreq     float64
		minTime     string
		maxTime     string
	)

	// File paths
	flag.StringVar(&c.DBPath, "db", "", "Path to the database file")
	flag.StringVar(&c.OutputFile, "o", "", "Path to the output file (without extension)")

	// Data selection
	flag.Int64Var(&c.SessionID, "s", 1, "Session ID")
	flag.Float64Var(&minFreq, "min-freq", 0, "Minimum frequency filter (Hz)")
	flag.Float64Var(&maxFreq, "max-freq", 0, "Maximum frequency filter (Hz)")
	flag.StringVar(&minTime, "min-time", "", "Minimum timestamp filter (RFC3339)")
	flag.StringVar(&maxTime, "max-time", "", "Maximum timestamp filter (RFC3339)")
	flag.Var(&timeZoneFlag{&c.TimeZone}, "tz", "Timezone for time display (e.g., 'America/New_York')")

	// Visualization
	flag.StringVar(&imageFormat, "f", string(ImagePNG), "Output image format [png, jpeg]")
	flag.StringVar(&theme, "theme", "", "Color theme [classic, grayscale, jungle, thermal, marine]")
	flag.Parse()

	// Validate and normalize input
	var errs []error

	// Required fields
	if c.DBPath == "" {
		errs = append(errs, errors.New("db path is required"))
	}
	if c.SessionID <= 0 {
		errs = append(errs, errors.New("session id is required"))
	}
	if c.OutputFile == "" {
		errs = append(errs, errors.New("output file is required"))
	}

	// Image format
	imageFormat = strings.ToLower(imageFormat)
	if _, ok := validImageFormats[ImageFormat(imageFormat)]; !ok {
		errs = append(errs, fmt.Errorf("invalid image format: %s", imageFormat))
	}

	// Theme
	theme = strings.ToLower(theme)
	if _, ok := validThemes[ColorTheme(theme)]; !ok {
		errs = append(errs, fmt.Errorf("invalid theme: %s", theme))
	}

	// Optional frequency filter
	if minFreq != 0 {
		if minFreq < 0 {
			errs = append(errs, errors.New("min-freq must be positive"))
		} else {
			c.MinFrequency = &minFreq
		}
	}
	if maxFreq != 0 {
		if maxFreq < 0 {
			errs = append(errs, errors.New("max-freq must be positive"))
		} else {
			c.MaxFrequency = &maxFreq
		}
	}
	if c.MinFrequency != nil && c.MaxFrequency != nil && *c.MinFrequency >= *c.MaxFrequency {
		errs = append(errs, errors.New("min-freq must be less than max-freq"))
	}

	// Optional time filter
	if minTime != "" {
		t, err := time.Parse(time.RFC3339, minTime)
		if err != nil {
			errs = append(errs, fmt.Errorf("invalid min-time: %w", err))
		} else {
			c.MinTimestamp = &t
		}
	}
	if maxTime != "" {
		t, err := time.Parse(time.RFC3339, maxTime)
		if err != nil {
			errs = append(errs, fmt.Errorf("invalid max-time: %w", err))
		} else {
			c.MaxTimestamp = &t
		}
	}
	if c.MinTimestamp != nil && c.MaxTimestamp != nil && c.MinTimestamp.After(*c.MaxTimestamp) {
		errs = append(errs, errors.New("min-time must be before max-time"))
	}

	if len(errs) > 0 {
		flag.Usage()
		return nil, fmt.Errorf("%w: %w", ErrInvalidConfig, errors.Join(errs...))
	}

	// Set validated values
	c.Format = ImageFormat(imageFormat)
	c.Theme = ColorTheme(theme)
	c.OutputFile = fmt.Sprintf("%s.%s", c.OutputFile, c.Format)

	return c, nil
}
