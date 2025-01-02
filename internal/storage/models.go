package storage

import (
	"database/sql"
	"time"

	"github.com/roman-kulish/radio-surveillance/internal/sdr"
	"github.com/roman-kulish/radio-surveillance/internal/telemetry"
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
	GroundSpeed  sql.NullInt64
	GroundCourse sql.NullInt64
	RadioRSSI    sql.NullInt64
}

func toTelemetryData(sessionID int64, t *telemetry.Telemetry) *telemetryData {
	return &telemetryData{
		SessionID: sessionID,
		Timestamp: t.Timestamp.UTC(),

		Latitude: sql.NullFloat64{
			Float64: toNullableNumber[float64](t.Latitude),
			Valid:   t.Latitude != nil,
		},
		Longitude: sql.NullFloat64{
			Float64: toNullableNumber[float64](t.Longitude),
			Valid:   t.Longitude != nil,
		},
		Altitude: sql.NullFloat64{
			Float64: toNullableNumber[float64](t.Altitude),
			Valid:   t.Altitude != nil,
		},
		Roll: sql.NullFloat64{
			Float64: toNullableNumber[float64](t.Roll),
			Valid:   t.Roll != nil,
		},
		Pitch: sql.NullFloat64{
			Float64: toNullableNumber[float64](t.Pitch),
			Valid:   t.Pitch != nil,
		},
		Yaw: sql.NullFloat64{
			Float64: toNullableNumber[float64](t.Yaw),
			Valid:   t.Yaw != nil,
		},
		AccelX: sql.NullFloat64{
			Float64: toNullableNumber[float64](t.AccelX),
			Valid:   t.AccelX != nil,
		},
		AccelY: sql.NullFloat64{
			Float64: toNullableNumber[float64](t.AccelY),
			Valid:   t.AccelY != nil,
		},
		AccelZ: sql.NullFloat64{
			Float64: toNullableNumber[float64](t.AccelZ),
			Valid:   t.AccelZ != nil,
		},
		GroundSpeed: sql.NullInt64{
			Int64: toNullableNumber[int64](t.GroundSpeed),
			Valid: t.GroundSpeed != nil,
		},
		GroundCourse: sql.NullInt64{
			Int64: toNullableNumber[int64](t.GroundCourse),
			Valid: t.GroundCourse != nil,
		},
		RadioRSSI: sql.NullInt64{
			Int64: toNullableNumber[int64](t.RadioRSSI),
			Valid: t.RadioRSSI != nil,
		},
	}
}

func toSampleData(sessionID int64, telemetryID *int64, r sdr.PowerReading, sr *sdr.SweepResult) *sampleData {
	var power sql.NullFloat64
	if r.IsValid {
		power.Float64 = r.Power
		power.Valid = true
	}

	var tmID sql.NullInt64
	if telemetryID != nil {
		tmID.Int64 = *telemetryID
		tmID.Valid = true
	}

	return &sampleData{
		SessionID:   sessionID,
		Timestamp:   sr.Timestamp.UTC(),
		Frequency:   r.Frequency,
		BinWidth:    sr.BinWidth,
		Power:       power,
		NumSamples:  sr.NumSamples,
		TelemetryID: tmID,
	}
}

func toNullableNumber[T float64 | int64, Y float64 | int](f *Y) T {
	if f == nil {
		return 0
	}
	return T(*f)
}
