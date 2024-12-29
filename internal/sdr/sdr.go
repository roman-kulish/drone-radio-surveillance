package sdr

import "time"

// Sample represents a single sample from the SDR
type Sample struct {
	Timestamp  time.Time // Timestamp information
	Frequency  float64   // Center frequency in Hz
	Power      float64   // Power level (dBm for rtl_sdr, dB for hackrf)
	BinWidth   float64   // Hz step/bin width
	NumSamples int       // Number of samples used for this measurement
	Device     string    // Device type (e.g., "rtl-sdr", "hackrf")
	DeviceID   string    // Serial number or index (human-readable)
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
