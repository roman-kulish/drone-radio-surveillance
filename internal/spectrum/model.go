package spectrum

import (
	"time"
)

// ScanSession represents a single spectrum scanning session with a specific device.
// Each session captures metadata about when and how the scanning was performed.
type ScanSession struct {
	ID         int64     // Unique identifier for the session
	StartTime  time.Time // When the scanning session began
	DeviceType string    // Type of SDR device used (e.g., "rtl-sdr", "hackrf")
	DeviceID   string    // Unique identifier of the specific device (e.g., serial number)
	Config     *string   // Optional device configuration in JSON format
}

// SpectralPoint represents a single measurement at a specific frequency.
// It captures the power level and measurement parameters for that frequency point.
type SpectralPoint struct {
	Frequency  float64  // Center frequency in Hz
	Power      *float64 // Measured power level in dBm (nil if measurement invalid)
	BinWidth   float64  // Frequency bin width in Hz
	NumSamples int      // Number of samples used for this measurement
}

// SpectralPointWithTelemetry extends SpectralPoint with drone telemetry data,
// associating spectrum measurements with the drone's state and position.
type SpectralPointWithTelemetry struct {
	SpectralPoint
	Altitude     *float64 // Barometric altitude in meters
	Roll         *float64 // Roll angle in degrees
	Pitch        *float64 // Pitch angle in degrees
	Yaw          *float64 // Yaw angle in degrees
	AccelX       *float64 // X-axis acceleration in m/s²
	AccelY       *float64 // Y-axis acceleration in m/s²
	AccelZ       *float64 // Z-axis acceleration in m/s²
	Latitude     *float64 // GPS latitude in degrees
	Longitude    *float64 // GPS longitude in degrees
	GroundSpeed  *float64 // Ground speed in m/s
	GroundCourse *float64 // Ground course (heading) in degrees
	RadioRSSI    *int64   // Radio link RSSI in dBm
}

// SpectralSpan represents a complete spectrum measurement at a point in time.
// It contains a sequence of measurements across a frequency range, optionally
// including telemetry data for each point.
type SpectralSpan[T SpectralPoint | SpectralPointWithTelemetry] struct {
	Timestamp time.Time // When this span of measurements was taken
	StartFreq float64   // Start frequency of the span in Hz
	EndFreq   float64   // End frequency of the span in Hz
	Samples   []T       // Ordered sequence of measurements in this span
}
