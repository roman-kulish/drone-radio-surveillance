package storage

import (
	"database/sql"
	"time"

	"github.con/roman-kulish/radio-surveillance/internal/sdr/hackrf"
	"github.con/roman-kulish/radio-surveillance/internal/sdr/rtl"
	"github.con/roman-kulish/radio-surveillance/internal/telemetry"
)

type Config interface {
	*rtl.Config | *hackrf.Config
}

type SessionData[T Config] struct {
	ID         int64
	StartTime  time.Time
	DeviceType string
	DeviceID   string
	Config     T
}

// SampleData represents a single frequency measurement
type SampleData struct {
	SessionID   int64
	Timestamp   time.Time
	Frequency   float64
	BinWidth    float64
	Power       sql.NullFloat64
	NumSamples  int
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
	GroundSpeed  sql.NullInt16
	GroundCourse sql.NullInt16
	RadioRSSI    sql.NullInt16
}

func (td *TelemetryData) FromTelemetry(t *telemetry.Telemetry) {
	td.Timestamp = t.Timestamp

	td.Latitude = sql.NullFloat64{
		Float64: *t.Latitude,
		Valid:   t.Latitude != nil,
	}

	td.Longitude = sql.NullFloat64{
		Float64: *t.Longitude,
		Valid:   t.Longitude != nil,
	}

	td.Altitude = sql.NullFloat64{
		Float64: *t.Altitude,
		Valid:   t.Altitude != nil,
	}

	td.Roll = sql.NullFloat64{
		Float64: *t.Roll,
		Valid:   t.Roll != nil,
	}

	td.Pitch = sql.NullFloat64{
		Float64: *t.Pitch,
		Valid:   t.Pitch != nil,
	}

	td.Yaw = sql.NullFloat64{
		Float64: *t.Yaw,
		Valid:   t.Yaw != nil,
	}

	td.AccelX = sql.NullFloat64{
		Float64: *t.AccelX,
		Valid:   t.AccelX != nil,
	}
	td.AccelY = sql.NullFloat64{
		Float64: *t.AccelY,
		Valid:   t.AccelY != nil,
	}

	td.AccelZ = sql.NullFloat64{
		Float64: *t.AccelZ,
		Valid:   t.AccelZ != nil,
	}

	td.GroundSpeed = sql.NullInt16{
		Int16: int16(*t.GroundSpeed),
		Valid: t.GroundSpeed != nil,
	}

	td.GroundCourse = sql.NullInt16{
		Int16: int16(*t.GroundCourse),
		Valid: t.GroundCourse != nil,
	}

	td.RadioRSSI = sql.NullInt16{
		Int16: int16(*t.RadioRSSI),
		Valid: t.RadioRSSI != nil,
	}
}
