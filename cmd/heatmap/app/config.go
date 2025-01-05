package app

import (
	"errors"
	"flag"
	"fmt"
	"strings"
)

const (
	ImagePNG  ImageFormat = "png"
	ImageJPEG ImageFormat = "jpeg"
)

type ImageFormat string

type Config struct {
	DBPath        string
	SessionID     int64
	Theme         ColorTheme
	OutputFile    string
	Format        ImageFormat
	Verbose       bool
	NoAnnotations bool
}

var validImageFormats = map[ImageFormat]struct{}{
	ImagePNG:  {},
	ImageJPEG: {},
}

var validThemes = map[ColorTheme]struct{}{
	ColorTheme(""): {},
	ClassicTheme:   {},
	GrayscaleTheme: {},
	JungleTheme:    {},
	ThermalTheme:   {},
	MarineTheme:    {},
}

func NewConfig() *Config {
	return &Config{
		Format: ImagePNG,
		Theme:  "",
	}
}

func NewConfigFromCLI() (*Config, error) {
	c := NewConfig()

	var imageFormat, theme string
	flag.StringVar(&c.DBPath, "db", "", "Path to the database file")
	flag.Int64Var(&c.SessionID, "s", 1, "Session ID")
	flag.StringVar(&c.OutputFile, "o", "", "Path to the output file")
	flag.StringVar(&imageFormat, "f", string(ImagePNG), "Output image format. [png, jpeg]")
	flag.StringVar(&theme, "theme", "", "Color theme. [classic, grayscale, jungle, thermal, marine]")
	flag.BoolVar(&c.Verbose, "verbose", false, "Enable more verbose output")
	flag.BoolVar(&c.NoAnnotations, "no-annotations", false, "Disabled annotations such as time and frequency scales")
	flag.Parse()

	imageFormat = strings.ToLower(imageFormat)
	theme = strings.ToLower(theme)

	var err error
	switch {
	case c.DBPath == "":
		err = errors.New("db path is required")
	case c.SessionID <= 0:
		err = errors.New("session id is required")
	case c.OutputFile == "":
		err = errors.New("output file is required")
	}
	if _, ok := validImageFormats[ImageFormat(imageFormat)]; !ok {
		err = fmt.Errorf("invalid image format: %s", imageFormat)
	}
	if _, ok := validThemes[ColorTheme(theme)]; !ok {
		err = fmt.Errorf("invalid theme: %s", theme)
	}
	if err != nil {
		flag.Usage()
		return nil, err
	}

	c.Format = ImageFormat(imageFormat)
	c.Theme = ColorTheme(theme)
	c.OutputFile = fmt.Sprintf("%s.%s", c.OutputFile, c.Format)
	return c, nil
}
