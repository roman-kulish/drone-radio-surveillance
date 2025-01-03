package storage

import (
	"database/sql"
	"math"

	"github.com/roman-kulish/radio-surveillance/internal/sdr"
	"github.com/roman-kulish/radio-surveillance/internal/telemetry"
)

func closeWithError(cl interface{ Close() error }, err *error) {
	if cErr := cl.Close(); cErr != nil && *err == nil {
		*err = cErr
	}
}

func rollbackWithError(rb interface{ Rollback() error }, err *error) {
	if cErr := rb.Rollback(); cErr != nil && *err == nil {
		*err = cErr
	}
}

func toTelemetryData(sessionID int64, t *telemetry.Telemetry) *telemetryData {
	return &telemetryData{
		SessionID: sessionID,
		Timestamp: t.Timestamp.UTC(),

		Latitude: sql.NullFloat64{
			Float64: toSQLNullType[float64](t.Latitude),
			Valid:   t.Latitude != nil,
		},
		Longitude: sql.NullFloat64{
			Float64: toSQLNullType[float64](t.Longitude),
			Valid:   t.Longitude != nil,
		},
		Altitude: sql.NullFloat64{
			Float64: toSQLNullType[float64](t.Altitude),
			Valid:   t.Altitude != nil,
		},
		Roll: sql.NullFloat64{
			Float64: toSQLNullType[float64](t.Roll),
			Valid:   t.Roll != nil,
		},
		Pitch: sql.NullFloat64{
			Float64: toSQLNullType[float64](t.Pitch),
			Valid:   t.Pitch != nil,
		},
		Yaw: sql.NullFloat64{
			Float64: toSQLNullType[float64](t.Yaw),
			Valid:   t.Yaw != nil,
		},
		AccelX: sql.NullFloat64{
			Float64: toSQLNullType[float64](t.AccelX),
			Valid:   t.AccelX != nil,
		},
		AccelY: sql.NullFloat64{
			Float64: toSQLNullType[float64](t.AccelY),
			Valid:   t.AccelY != nil,
		},
		AccelZ: sql.NullFloat64{
			Float64: toSQLNullType[float64](t.AccelZ),
			Valid:   t.AccelZ != nil,
		},
		GroundSpeed: sql.NullFloat64{
			Float64: toSQLNullType[float64](t.GroundSpeed),
			Valid:   t.GroundSpeed != nil,
		},
		GroundCourse: sql.NullFloat64{
			Float64: toSQLNullType[float64](t.GroundCourse),
			Valid:   t.GroundCourse != nil,
		},
		RadioRSSI: sql.NullInt64{
			Int64: toSQLNullType[int64](t.RadioRSSI),
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

func toSQLNullType[T float64 | int64, Y float64 | int | int64](f *Y) T {
	if f == nil {
		return 0
	}
	return T(*f)
}

// freqCompare helps compare frequencies using bin width-based tolerance.
// Returns:
//
//	-1 if a < b
//	 0 if a ≈ b (within tolerance)
//	+1 if a > b
func freqCompare(a, b, binWidth float64) int {
	// Use small fraction of bin width as tolerance
	tolerance := binWidth * 0.01 // 1% of bin width

	diff := a - b
	if math.Abs(diff) <= tolerance {
		return 0
	}
	if diff < 0 {
		return -1
	}
	return 1
}

// freqLess returns true if a is less than b with bin width-based tolerance
func freqLess(a, b, binWidth float64) bool {
	return freqCompare(a, b, binWidth) < 0
}

// freqEqual returns true if a equals b within bin width-based tolerance
func freqEqual(a, b, binWidth float64) bool {
	return freqCompare(a, b, binWidth) == 0
}

// freqGreater returns true if a is greater than b with bin width-based tolerance
func freqGreater(a, b, binWidth float64) bool {
	return freqCompare(a, b, binWidth) > 0
}
