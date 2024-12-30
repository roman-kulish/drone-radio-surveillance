package app

import (
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

type TelemetryType string

type DeviceTypeConfig interface {
	*rtl.Config | *hackrf.Config
}

// Config represents the main application configuration
type Config[T DeviceTypeConfig] struct {
	Settings  Settings          `yaml:"settings"`
	Devices   []DeviceConfig[T] `yaml:"devices"`
	Telemetry TelemetryConfig   `yaml:"telemetry"`
	Storage   StorageConfig     `yaml:"storage"`
}

// Settings represents global application settings
type Settings struct {
	LogLevel string `yaml:"logLevel"`
}

// DeviceConfig represents a single device configuration
type DeviceConfig[T DeviceTypeConfig] struct {
	Name    string `yaml:"name"`
	Type    string `yaml:"type"`
	Enabled bool   `yaml:"enabled"`
	Config  T      `yaml:"configFile"`
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
