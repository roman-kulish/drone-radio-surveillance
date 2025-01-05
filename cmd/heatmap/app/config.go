package app

import (
	"errors"
	"flag"
	"fmt"
	"strings"
)

const (
	ImagePNG  = "png"
	ImageJPEG = "jpeg"
)

type ImageFormat string

type Config struct {
	DBPath        string
	SessionID     int64
	OutputFile    string
	Format        ImageFormat
	MaxPower      *float64
	MinPower      *float64
	Verbose       bool
	NoAnnotations bool
}

var validImageFormats = map[ImageFormat]struct{}{
	ImagePNG:  {},
	ImageJPEG: {},
}

func NewConfig() *Config {
	return &Config{
		Format: ImagePNG,
	}
}

func NewConfigFromCLI() (*Config, error) {
	c := NewConfig()

	var imageFormat string
	var minPower, maxPower float64
	flag.StringVar(&c.DBPath, "db", "", "Path to the database file")
	flag.Int64Var(&c.SessionID, "s", 1, "Session ID")
	flag.StringVar(&c.OutputFile, "o", "", "Path to the output file")
	flag.StringVar(&imageFormat, "f", string(ImagePNG), "Output image format. [png, jpeg]")
	flag.Float64Var(&minPower, "min-power", 0, "Define a manual minimum power (format nn.n)")
	flag.Float64Var(&maxPower, "max-power", 0, "Define a manual maximum power (format nn.n)")
	flag.BoolVar(&c.Verbose, "verbose", false, "Enable more verbose output")
	flag.BoolVar(&c.NoAnnotations, "no-annotations", false, "Disabled annotations such as time and frequency scales")
	flag.Parse()

	imageFormat = strings.ToLower(imageFormat)

	flag.Visit(func(f *flag.Flag) {
		if f.Name == "min-power" {
			c.MinPower = &minPower
		}
		if f.Name == "max-power" {
			c.MaxPower = &maxPower
		}
	})

	var err error
	if c.DBPath == "" {
		err = errors.New("db path is required")
	} else if c.SessionID <= 0 {
		err = errors.New("session id is required")
	} else if c.OutputFile == "" {
		err = errors.New("output file is required")
	} else if _, ok := validImageFormats[ImageFormat(imageFormat)]; !ok {
		err = fmt.Errorf("invalid image format: %s", imageFormat)
	}

	if err != nil {
		flag.Usage()
		return nil, err
	}

	c.Format = ImageFormat(imageFormat)
	c.OutputFile = fmt.Sprintf("%s.%s", c.OutputFile, c.Format)
	return c, nil
}
