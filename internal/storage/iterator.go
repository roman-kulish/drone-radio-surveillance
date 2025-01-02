package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/roman-kulish/radio-surveillance/internal/spectrum"
	"github.com/roman-kulish/radio-surveillance/internal/telemetry"
)

// SpectralData is a constraint for types that can represent spectrum measurements,
// either basic spectral points or those enriched with telemetry data.
type SpectralData interface {
	spectrum.SpectralPoint | spectrum.SpectralPointWithTelemetry

	CentralFrequency() float64
}

// SpectrumReader provides an iterator-based interface for reading spectrum data
// with optional time and frequency filtering. The type parameter T determines whether
// the reader returns basic spectral points or points with telemetry data.
type SpectrumReader[T SpectralData] interface {
	// Session returns metadata about the capture session this reader is accessing.
	// This includes device information, timing, and configuration details.
	Session() *spectrum.ScanSession

	// Next advances the iterator and returns true if there is another spectrum point
	// to read, false when the iteration is complete or if an error occurred.
	Next(context.Context) bool

	// Current returns the current spectral span in the iteration.
	// If called after Next() returns false, the behavior is undefined.
	Current() *spectrum.SpectralSpan[T]

	// Error returns any error that occurred during iteration.
	// If Next() returns false, Err() should be checked to distinguish between
	// end of data and an error condition.
	Error() error

	// Close releases any resources associated with the reader.
	// After Close is called, the reader should not be used.
	Close() error
}

// ReaderOption configures a SpectrumReader with specific filtering criteria.
// The type parameter T must match the reader being configured.
type ReaderOption[T SpectralData] func(*spectrumReader[T])

// WithMinFreq sets the minimum frequency filter for the spectrum reader.
// Spectrum points with frequencies below this value will be excluded.
func WithMinFreq[T SpectralData](f float64) ReaderOption[T] {
	return func(r *spectrumReader[T]) {
		r.minFreq = &f
	}
}

// WithMaxFreq sets the maximum frequency filter for the spectrum reader.
// Spectrum points with frequencies above this value will be excluded.
func WithMaxFreq[T SpectralData](f float64) ReaderOption[T] {
	return func(r *spectrumReader[T]) {
		r.maxFreq = &f
	}
}

// WithFreqRange sets both minimum and maximum frequency filters.
// This is a convenience function equivalent to applying both WithMinFreq
// and WithMaxFreq.
func WithFreqRange[T SpectralData](minFreq, maxFreq float64) ReaderOption[T] {
	return func(r *spectrumReader[T]) {
		r.minFreq = &minFreq
		r.maxFreq = &maxFreq
	}
}

// WithStartTime sets the start time filter for the spectrum reader.
// Spectrum points with timestamps before this time will be excluded.
func WithStartTime[T SpectralData](t time.Time) ReaderOption[T] {
	return func(r *spectrumReader[T]) {
		r.startTime = &t
	}
}

// WithEndTime sets the end time filter for the spectrum reader.
// Spectrum points with timestamps after this time will be excluded.
func WithEndTime[T SpectralData](t time.Time) ReaderOption[T] {
	return func(r *spectrumReader[T]) {
		r.endTime = &t
	}
}

// WithTimeRange sets both start and end time filters.
// This is a convenience function equivalent to applying both WithStartTime
// and WithEndTime.
func WithTimeRange[T SpectralData](startTime, endTime time.Time) ReaderOption[T] {
	return func(r *spectrumReader[T]) {
		r.startTime = &startTime
		r.endTime = &endTime
	}
}

// spectrumReader implements SpectrumReader for SQLite database backend.
type spectrumReader[T SpectralData] struct {
	db *sql.DB

	sessionID        int64
	session          *spectrum.ScanSession
	includeTelemetry bool

	startTime *time.Time // Optional start of time range filter
	endTime   *time.Time // Optional end of time range filter
	minFreq   *float64   // Optional minimum frequency filter
	maxFreq   *float64   // Optional maximum frequency filter

	currentSpan            *spectrum.SpectralSpan[T]
	nextSample             T // First sample of next span
	nextSampleExists       bool
	nextSpanStartTimestamp time.Time
	rows                   *sql.Rows
	err                    error
}

func (sr *spectrumReader[T]) init(ctx context.Context) error {
	if sr.db == nil {
		return errors.New("database connection required")
	}
	if sr.sessionID <= 0 {
		return errors.New("session ID required")
	}
	for _, fn := range []func(context.Context) error{sr.loadSession, sr.initFilters, sr.initQuery} {
		if err := fn(ctx); err != nil {
			return fmt.Errorf("initializing reader: %w", err)
		}
	}
	return nil
}

func (sr *spectrumReader[T]) loadSession(ctx context.Context) (err error) {
	stmt, err := sr.db.PrepareContext(ctx, selectSessionSQL)
	if err != nil {
		return fmt.Errorf("preparing statement: %w", err)
	}
	defer closeWithError(stmt, &err)

	var sess spectrum.ScanSession
	var config sql.NullString
	if err = stmt.QueryRowContext(ctx, sr.sessionID).Scan(&sess.ID, &sess.StartTime, &sess.DeviceType, &sess.DeviceID, &config); err != nil {
		return fmt.Errorf("querying session: %w", err)
	}
	if config.Valid {
		sess.Config = &config.String
	}

	sr.session = &sess
	return
}

func (sr *spectrumReader[T]) initFilters(ctx context.Context) (err error) {
	timeFiltersSet := sr.startTime != nil && sr.endTime != nil
	freqFiltersSet := sr.minFreq != nil && sr.maxFreq != nil

	if timeFiltersSet {
		if sr.startTime.After(*sr.endTime) {
			return fmt.Errorf("start time %s is after end time %s", sr.startTime, sr.endTime)
		}
	}
	if freqFiltersSet {
		if *sr.minFreq > *sr.maxFreq {
			return fmt.Errorf("min frequency %f is greater than max frequency %f", *sr.minFreq, *sr.maxFreq)
		}
	}
	if timeFiltersSet && freqFiltersSet {
		return nil
	}

	stmt, err := sr.db.PrepareContext(ctx, selectFilterValuesSQL)
	if err != nil {
		return fmt.Errorf("preparing statement: %w", err)
	}
	defer closeWithError(stmt, &err)

	var minFreq, maxFreq float64
	var startTime, endTime time.Time
	if err = stmt.QueryRowContext(ctx, sr.sessionID).Scan(&minFreq, &maxFreq, &startTime, &endTime); err != nil {
		return fmt.Errorf("scanning session: %w", err)
	}

	if sr.minFreq == nil {
		sr.minFreq = &minFreq
	}
	if sr.maxFreq == nil {
		sr.maxFreq = &maxFreq
	}
	if sr.startTime == nil {
		sr.startTime = &startTime
	}
	if sr.endTime == nil {
		sr.endTime = &endTime
	}

	return nil
}

func (sr *spectrumReader[T]) initQuery(ctx context.Context) (err error) {
	query := selectSamplesSQL
	if sr.includeTelemetry {
		query = selectSamplesWithTelemetrySQL
	}

	stmt, err := sr.db.PrepareContext(ctx, query)
	if err != nil {
		return fmt.Errorf("preparing statement: %w", err)
	}
	defer closeWithError(stmt, &err)

	if sr.rows, err = stmt.QueryContext(ctx, sr.sessionID, sr.startTime, sr.endTime, sr.minFreq, sr.maxFreq); err != nil {
		return err
	}
	return nil
}

func (sr *spectrumReader[T]) scanSample() (time.Time, T, error) {
	var zero T

	var sample sampleData
	var timestamp time.Time

	err := sr.rows.Scan(&timestamp, &sample.Frequency, &sample.Power, &sample.BinWidth, &sample.NumSamples)
	if err != nil {
		return time.Time{}, zero, fmt.Errorf("scanning sample: %w", err)
	}

	var power *float64
	if sample.Power.Valid {
		power = &sample.Power.Float64
	}

	point := spectrum.SpectralPoint{
		Frequency:  sample.Frequency,
		Power:      power,
		BinWidth:   sample.BinWidth,
		NumSamples: sample.NumSamples,
	}

	result, ok := any(point).(T)
	if !ok {
		return time.Time{}, zero, fmt.Errorf("invalid type conversion from %T to %T", point, zero)
	}
	return timestamp, result, nil
}

func (sr *spectrumReader[T]) scanSampleWithTelemetry() (time.Time, T, error) {
	var zero T

	var sample sampleWithTelemetryData
	var timestamp time.Time
	err := sr.rows.Scan(
		&timestamp,
		&sample.Frequency,
		&sample.Power,
		&sample.BinWidth,
		&sample.NumSamples,
		&sample.TelemetryID,
		&sample.Latitude,
		&sample.Longitude,
		&sample.Altitude,
		&sample.Roll,
		&sample.Pitch,
		&sample.Yaw,
		&sample.AccelX,
		&sample.AccelY,
		&sample.AccelZ,
		&sample.GroundSpeed,
		&sample.GroundCourse,
		&sample.RadioRSSI,
	)
	if err != nil {
		return time.Time{}, zero, fmt.Errorf("scanning sample: %w", err)
	}

	var power *float64
	if sample.Power.Valid {
		power = &sample.Power.Float64
	}

	point := spectrum.SpectralPointWithTelemetry{
		SpectralPoint: spectrum.SpectralPoint{
			Frequency:  sample.Frequency,
			Power:      power,
			BinWidth:   sample.BinWidth,
			NumSamples: sample.NumSamples,
		},
	}

	if !sample.TelemetryID.Valid {
		result, ok := any(point).(T)
		if !ok {
			return time.Time{}, zero, fmt.Errorf("invalid type conversion from %T to %T", point, zero)
		}
		return timestamp, result, nil
	}

	point.Telemetry = &telemetry.Telemetry{}

	if sample.Latitude.Valid {
		point.Telemetry.Latitude = &sample.Latitude.Float64
	}
	if sample.Longitude.Valid {
		point.Telemetry.Longitude = &sample.Longitude.Float64
	}
	if sample.Altitude.Valid {
		point.Telemetry.Altitude = &sample.Altitude.Float64
	}
	if sample.Roll.Valid {
		point.Telemetry.Roll = &sample.Roll.Float64
	}
	if sample.Pitch.Valid {
		point.Telemetry.Pitch = &sample.Pitch.Float64
	}
	if sample.Yaw.Valid {
		point.Telemetry.Yaw = &sample.Yaw.Float64
	}
	if sample.AccelX.Valid {
		point.Telemetry.AccelX = &sample.AccelX.Float64
	}
	if sample.AccelY.Valid {
		point.Telemetry.AccelY = &sample.AccelY.Float64
	}
	if sample.AccelZ.Valid {
		point.Telemetry.AccelZ = &sample.AccelZ.Float64
	}
	if sample.GroundSpeed.Valid {
		point.Telemetry.GroundSpeed = &sample.GroundSpeed.Float64
	}
	if sample.GroundCourse.Valid {
		point.Telemetry.GroundCourse = &sample.GroundCourse.Float64
	}
	if sample.RadioRSSI.Valid {
		point.Telemetry.RadioRSSI = &sample.RadioRSSI.Int64
	}

	result, ok := any(point).(T)
	if !ok {
		return time.Time{}, zero, fmt.Errorf("invalid type conversion from %T to %T", point, zero)
	}
	return timestamp, result, nil
}

func (sr *spectrumReader[T]) Session() *spectrum.ScanSession {
	return sr.session
}

func (sr *spectrumReader[T]) Next(ctx context.Context) bool {
	if sr.err != nil || sr.rows == nil {
		return false
	}

	if sr.nextSampleExists {
		sr.currentSpan = &spectrum.SpectralSpan[T]{
			Timestamp: sr.nextSpanStartTimestamp,
			StartFreq: sr.nextSample.CentralFrequency(),
			Samples:   []T{sr.nextSample},
		}
		sr.nextSampleExists = false
	}

	for {
		select {
		case <-ctx.Done():
			sr.err = ctx.Err()
			return false
		default:
		}

		if !sr.rows.Next() {
			if sr.currentSpan != nil {
				if len(sr.currentSpan.Samples) > 0 {
					lastSample := sr.currentSpan.Samples[len(sr.currentSpan.Samples)-1]
					sr.currentSpan.EndFreq = lastSample.CentralFrequency()
				}
				return true
			}
			return false
		}

		var timestamp time.Time
		var sample T
		if sr.includeTelemetry {
			timestamp, sample, sr.err = sr.scanSampleWithTelemetry()
		} else {
			timestamp, sample, sr.err = sr.scanSample()
		}
		if sr.err != nil {
			return false
		}

		// If no current span, create new one
		if sr.currentSpan == nil {
			sr.currentSpan = &spectrum.SpectralSpan[T]{
				Timestamp: timestamp,
				StartFreq: sample.CentralFrequency(),
				Samples:   []T{sample},
			}
			continue
		}

		// Check for frequency rollover only
		lastSample := sr.currentSpan.Samples[len(sr.currentSpan.Samples)-1]
		if sample.CentralFrequency() < lastSample.CentralFrequency() {
			// Frequency rolled over - complete current span
			sr.currentSpan.EndFreq = lastSample.CentralFrequency()
			sr.nextSample = sample
			sr.nextSampleExists = true
			sr.nextSpanStartTimestamp = timestamp
			return true
		}

		// Add sample to current span
		sr.currentSpan.Samples = append(sr.currentSpan.Samples, sample)
	}
}

func (sr *spectrumReader[T]) Current() *spectrum.SpectralSpan[T] {
	return sr.currentSpan
}

func (sr *spectrumReader[T]) Error() error {
	if sr.err != nil {
		return sr.err
	}
	if sr.rows != nil {
		return sr.rows.Err()
	}
	return nil
}

func (sr *spectrumReader[T]) Close() error {
	if sr.rows != nil {
		err := sr.rows.Close()
		sr.rows = nil
		return err
	}
	return nil
}
