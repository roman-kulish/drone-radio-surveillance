package app

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/roman-kulish/radio-surveillance/internal/sdr/hackrf"
	"github.com/roman-kulish/radio-surveillance/internal/sdr/rtl"
	"gopkg.in/yaml.v3"
)

const (
	TelemetryGPS          TelemetryType = "gps"
	TelemetryIMU          TelemetryType = "imu"
	TelemetryRadio        TelemetryType = "radio"
	TelemetryBarometer    TelemetryType = "barometer"
	TelemetryMagnetometer TelemetryType = "magnetometer"

	DeviceRTLSDR DeviceType = "rtl-sdr"
	DeviceHackRF DeviceType = "hackrf"
)

type TelemetryType string

type DeviceType string

// yamlNode is a custom type for unmarshalling raw YAML nodes
type yamlNode struct {
	*yaml.Node
}

// UnmarshalYAML implements the yaml.Unmarshaler interface for custom deserialization of yamlNode from YAML input.
func (y *yamlNode) UnmarshalYAML(value *yaml.Node) error {
	y.Node = value
	return nil
}

// Config represents the main application configuration
type Config struct {
	Settings  Settings        `yaml:"settings"`
	Devices   []DeviceConfig  `yaml:"devices"`
	Telemetry TelemetryConfig `yaml:"telemetry"`
	Storage   StorageConfig   `yaml:"storage"`
}

// Settings represents global application settings
type Settings struct {
	LogLevel slog.Level `yaml:"logLevel"`
}

func (s *Settings) UnmarshalYAML(value *yaml.Node) error {
	var t struct {
		LogLevel string `yaml:"logLevel"`
	}
	if err := value.Decode(&t); err != nil {
		return err
	}

	s.LogLevel = slog.LevelInfo
	return s.LogLevel.UnmarshalText([]byte(t.LogLevel))
}

// DeviceConfig represents a single Device configuration
type DeviceConfig struct {
	Name    string        `yaml:"name"`
	Type    DeviceType    `yaml:"type"`
	Enabled bool          `yaml:"enabled"`
	Config  any           `yaml:"config"`
	Buffer  *BufferConfig `yaml:"buffer"`
}

// UnmarshalYAML implements the yaml.Unmarshaler interface for custom deserialization of DeviceConfig from YAML input.
func (d *DeviceConfig) UnmarshalYAML(value *yaml.Node) error {
	var t struct {
		Name    string        `yaml:"name"`
		Type    DeviceType    `yaml:"type"`
		Enabled bool          `yaml:"enabled"`
		Config  yamlNode      `yaml:"config"`
		Buffer  *BufferConfig `yaml:"buffer"`
	}
	if err := value.Decode(&t); err != nil {
		return err
	}

	dc := DeviceConfig{
		Name:    t.Name,
		Type:    t.Type,
		Enabled: t.Enabled,
		Buffer:  t.Buffer,
	}
	switch t.Type {
	case DeviceRTLSDR:
		var c rtl.Config
		if err := t.Config.Decode(&c); err != nil {
			return err
		}

		dc.Config = &c

	case DeviceHackRF:
		var c hackrf.Config
		if err := t.Config.Decode(&c); err != nil {
			return err
		}

		dc.Config = &c

	default:
		return fmt.Errorf("unknown Device type: %s", t.Type)
	}

	*d = dc
	return nil
}

// TelemetryConfig represents telemetry settings
type TelemetryConfig struct {
	SerialPort      string          `yaml:"serialPort"`
	BaudRate        int             `yaml:"baudRate"`
	UpdateInterval  float64         `yaml:"updateInterval"`
	Enabled         bool            `yaml:"enabled"`
	CaptureInterval []string        `yaml:"captureInterval"`
	Types           []TelemetryType `yaml:"types"`
}

// BufferConfig represents device buffer settings
type BufferConfig struct {
	Capacity   int `yaml:"capacity"`
	FlushCount int `yaml:"flushCount"`
}

// StorageConfig represents storage settings
type StorageConfig struct {
	DataDirectory string `yaml:"dataDirectory"`
}

// LoadConfig reads a configuration file from the specified path and parses it into a Config struct.
func LoadConfig(path string) (*Config, error) {
	configFile, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading configuration file: %w", err)
	}

	var config Config
	if err = yaml.Unmarshal(configFile, &config); err != nil {
		return nil, fmt.Errorf("parsing configuration file: %w", err)
	}

	return &config, nil
}
