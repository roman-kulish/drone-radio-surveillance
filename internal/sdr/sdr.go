package sdr

import "time"

type Sample struct {
	// Timestamp information
	Timestamp time.Time

	// Frequency information
	FrequencyLow  int64 // Hz low from output
	FrequencyHigh int64 // Hz high from output
	BinWidth      int64 // Hz step/bin width

	// Measurement data
	Power      float64 // Power level (dBm for rtl_sdr, dB for hackrf)
	NumSamples int     // Number of samples used for this measurement

	// Device metadata
	Device   string // "rtl-sdr" or "hackrf"
	DeviceID string // Serial number or index
}

type Telemetry struct {
	Latitude      float64
	Longitude     float64
	Altitude      float64
	Roll          float64
	Pitch         float64
	Yaw           float64
	AccelX        float64
	AccelY        float64
	AccelZ        float64
	NumSatellites uint8
	GroundSpeed   uint16
	GroundCourse  uint16
	RadioRSSI     int
}

// FrequencyCenter is a helper method to get center frequency
func (s *Sample) FrequencyCenter() int64 {
	return s.FrequencyLow + (s.BinWidth / 2)
}

// CmdArgsBuilder is an interface for building command line arguments for SDR tools
type CmdArgsBuilder interface {
	Args() []string
}

type Device interface {
	Start() (<-chan Sample, error) // Returns read-only channel and error
	Stop() error                   // Graceful shutdown
	Status() DeviceStatus          // Current device state
	ID() string                    // Unique device identifier
}
