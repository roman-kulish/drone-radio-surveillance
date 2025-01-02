package telemetry

import (
	"time"
)

type Provider interface {
	Get() *Telemetry
}

// Telemetry is the telemetry data from the drone sensors
type Telemetry struct {
	Timestamp    time.Time // Timestamp of telemetry measurement
	Altitude     *float64  // Barometric altitude in meters
	Roll         *float64  // Roll angle in degrees
	Pitch        *float64  // Pitch angle in degrees
	Yaw          *float64  // Yaw angle in degrees
	AccelX       *float64  // X-axis acceleration in m/s²
	AccelY       *float64  // Y-axis acceleration in m/s²
	AccelZ       *float64  // Z-axis acceleration in m/s²
	Latitude     *float64  // GPS latitude in degrees
	Longitude    *float64  // GPS longitude in degrees
	GroundSpeed  *float64  // Ground speed in m/s
	GroundCourse *float64  // Ground course (heading) in degrees
	RadioRSSI    *int64    // Radio link RSSI in dBm
}
