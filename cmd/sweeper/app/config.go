package app

import (
	"fmt"
	"log/slog"
	"os"

	"gopkg.in/yaml.v3"
	"gtihub.con/roman-kulish/radio-surveillance/internal/sdr/hackrf"
	"gtihub.con/roman-kulish/radio-surveillance/internal/sdr/rtl"
)

const (
	TelemetryTypeGPS          TelemetryType = "gps"
	TelemetryTypeIMU          TelemetryType = "imu"
	TelemetryTypeRadio        TelemetryType = "radio"
	TelemetryTypeBarometer    TelemetryType = "barometer"
	TelemetryTypeMagnetometer TelemetryType = "magnetometer"
)

// yamlNode is a custom type for unmarshalling raw YAML nodes
type yamlNode struct {
	*yaml.Node
}

// UnmarshalYAML implements the yaml.Unmarshaler interface for custom deserialization of yamlNode from YAML input.
func (y *yamlNode) UnmarshalYAML(value *yaml.Node) error {
	y.Node = value
	return nil
}

type TelemetryType string

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

// DeviceConfig represents a single device configuration
type DeviceConfig struct {
	Name    string `yaml:"name"`
	Type    string `yaml:"type"`
	Enabled bool   `yaml:"enabled"`
	Config  any    `yaml:"config"`
}

// UnmarshalYAML implements the yaml.Unmarshaler interface for custom deserialization of DeviceConfig from YAML input.
func (d *DeviceConfig) UnmarshalYAML(value *yaml.Node) error {
	var t struct {
		Name    string   `yaml:"name"`
		Type    string   `yaml:"type"`
		Enabled bool     `yaml:"enabled"`
		Config  yamlNode `yaml:"config"`
	}
	if err := value.Decode(&t); err != nil {
		return err
	}

	dc := DeviceConfig{
		Name:    t.Name,
		Type:    t.Type,
		Enabled: t.Enabled,
	}
	switch t.Type {
	case "rtl-sdr":
		var c rtl.Config
		if err := t.Config.Decode(&c); err != nil {
			return err
		}

		dc.Config = &c

	case "hackrf":
		var c hackrf.Config
		if err := t.Config.Decode(&c); err != nil {
			return err
		}

		dc.Config = &c

	default:
		return fmt.Errorf("unknown device type: %s", t.Type)
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

// StorageConfig represents storage settings
type StorageConfig struct {
	DataDirectory string `yaml:"dataDirectory"`
	MaxBatchSize  int    `yaml:"maxBatchSize"`
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
