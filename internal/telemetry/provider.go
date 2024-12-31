package telemetry

import (
	"time"
)

type Provider interface {
	Get() *Telemetry
}

// Telemetry is the telemetry data from the drone sensors
type Telemetry struct {
	Timestamp time.Time

	// Barometer
	Altitude *float64

	// Gyroscope
	Roll  *float64
	Pitch *float64
	Yaw   *float64

	// Accelerometer
	AccelX *float64
	AccelY *float64
	AccelZ *float64

	// GPS
	Latitude    *float64
	Longitude   *float64
	GroundSpeed *uint16

	// Magnetometer
	GroundCourse *uint16

	// Radio
	RadioRSSI *int
}
