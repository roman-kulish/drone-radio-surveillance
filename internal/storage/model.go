package storage

import (
	"database/sql"
	"time"
)

type SessionData struct {
	ID         int64
	StartTime  time.Time
	DeviceType string
	DeviceID   string
	Config     sql.NullString
}

// SampleData represents a single frequency measurement
type SampleData struct {
	ID          int64
	SessionID   int64
	Timestamp   time.Time
	Frequency   float64
	BinWidth    float64
	Power       sql.NullFloat64
	NumSamples  int64
	TelemetryID sql.NullInt64
}

// TelemetryData represents drone telemetry data
type TelemetryData struct {
	ID           int64
	SessionID    int64
	Timestamp    time.Time
	Latitude     sql.NullFloat64
	Longitude    sql.NullFloat64
	Altitude     sql.NullFloat64
	Roll         sql.NullFloat64
	Pitch        sql.NullFloat64
	Yaw          sql.NullFloat64
	AccelX       sql.NullFloat64
	AccelY       sql.NullFloat64
	AccelZ       sql.NullFloat64
	GroundSpeed  sql.NullInt64
	GroundCourse sql.NullInt64
	RadioRSSI    sql.NullInt64
}
