package storage

import (
	"database/sql"
	"time"
)

type sampleData struct {
	ID          int64
	SessionID   int64
	Timestamp   time.Time
	Frequency   float64
	BinWidth    float64
	Power       sql.NullFloat64
	NumSamples  int
	TelemetryID sql.NullInt64
}

type telemetryData struct {
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
	GroundSpeed  sql.NullFloat64
	GroundCourse sql.NullFloat64
	RadioRSSI    sql.NullInt64
}

type sampleWithTelemetryData struct {
	sampleData
	telemetryData
}
