package spectrum

import (
	"time"

	"github.com/roman-kulish/radio-surveillance/internal/telemetry"
)

// ScanSession represents a single spectrum scanning session with a specific device.
// Each session captures metadata about when and how the scanning was performed.
type ScanSession struct {
	ID         int64     `json:"ID"`                      // Unique identifier for the session
	StartTime  time.Time `json:"startTime"`               // When the scanning session began
	DeviceType string    `json:"deviceType"`              // Type of SDR device used (e.g., "rtl-sdr", "hackrf")
	DeviceID   string    `json:"deviceID"`                // Unique identifier of the specific device (e.g., serial number)
	Config     *string   `json:"config,string,omitempty"` // Optional device configuration in JSON format
}

// SpectralPoint represents a single measurement at a specific frequency.
// It captures the power level and measurement parameters for that frequency point.
type SpectralPoint struct {
	Frequency  float64  `json:"frequency"`       // Center frequency in Hz
	Power      *float64 `json:"power,omitempty"` // Measured power level in dBm (nil if measurement invalid)
	BinWidth   float64  `json:"binWidth"`        // Frequency bin width in Hz
	NumSamples int      `json:"numSamples"`      // Number of samples used for this measurement
}

func (p SpectralPoint) GetFrequency() float64 {
	return p.Frequency
}

func (p SpectralPoint) GetBinWidth() float64 {
	return p.BinWidth
}

func (p SpectralPoint) GetNumSamples() int {
	return p.NumSamples
}

// SpectralPointWithTelemetry extends SpectralPoint with drone telemetry data,
// associating spectrum measurements with the drone's state and position.
type SpectralPointWithTelemetry struct {
	SpectralPoint `json:"spectralPoint"`
	Telemetry     *telemetry.Telemetry `json:"telemetry,omitempty"` // Drone telemetry data, if exists
}

// SpectralSpan represents a complete spectrum measurement at a point in time.
// It contains a sequence of measurements across a frequency range, optionally
// including telemetry data for each point.
type SpectralSpan[T SpectralPoint | SpectralPointWithTelemetry] struct {
	Timestamp      time.Time `json:"timestamp"`         // When this span of measurements was taken
	FrequencyStart float64   `json:"frequencyStart"`    // Start frequency of the span in Hz
	FrequencyEnd   float64   `json:"frequencyEnd"`      // End frequency of the span in Hz
	Samples        []T       `json:"samples,omitempty"` // Ordered sequence of measurements in this span
}
