package telemetry

import (
	"time"
)

type Provider interface {
	Get() *Telemetry
}

// Telemetry is the telemetry data from the drone sensors
type Telemetry struct {
	Timestamp    time.Time `json:"timestamp"`              // Timestamp of telemetry measurement
	Altitude     *float64  `json:"altitude,omitempty"`     // Barometric altitude in meters
	Roll         *float64  `json:"roll,omitempty"`         // Roll angle in degrees
	Pitch        *float64  `json:"pitch,omitempty"`        // Pitch angle in degrees
	Yaw          *float64  `json:"yaw,omitempty"`          // Yaw angle in degrees
	AccelX       *float64  `json:"accelX,omitempty"`       // X-axis acceleration in m/s²
	AccelY       *float64  `json:"accelY,omitempty"`       // Y-axis acceleration in m/s²
	AccelZ       *float64  `json:"accelZ,omitempty"`       // Z-axis acceleration in m/s²
	Latitude     *float64  `json:"latitude,omitempty"`     // GPS latitude in degrees
	Longitude    *float64  `json:"longitude,omitempty"`    // GPS longitude in degrees
	GroundSpeed  *float64  `json:"groundSpeed,omitempty"`  // Ground speed in m/s
	GroundCourse *float64  `json:"groundCourse,omitempty"` // Ground course (heading) in degrees
	RadioRSSI    *int64    `json:"radioRSSI,omitempty"`    // Radio link RSSI in dBm
}
