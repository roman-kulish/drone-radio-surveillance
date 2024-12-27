package sdr

import "time"

// Sample represents a single sample from the SDR
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

// FrequencyCenter is a helper method to get center frequency
func (s *Sample) FrequencyCenter() int64 {
	return s.FrequencyLow + (s.BinWidth / 2)
}

// // Telemetry is the telemetry data from the drone sensors
// type Telemetry struct {
// 	// Barometer
// 	Altitude float64
//
// 	// Gyroscope
// 	Roll  float64
// 	Pitch float64
// 	Yaw   float64
//
// 	// Accelerometer
// 	AccelX float64
// 	AccelY float64
// 	AccelZ float64
//
// 	// GPS
// 	Latitude    float64
// 	Longitude   float64
// 	GroundSpeed uint16
//
// 	// Magnetometer
// 	GroundCourse uint16
//
// 	// Radio
// 	RadioRSSI int
// }
//
// // EnrichedSample is a Sample with telemetry
// type EnrichedSample struct {
// 	Sample
// 	Telemetry
// }

// type Device interface {
// 	Start() (<-chan Sample, error) // Returns read-only channel and error
// 	Stop() error                   // Graceful shutdown
// 	Status() DeviceStatus          // Current device state
// 	ID() string                    // Unique device identifier
// }
