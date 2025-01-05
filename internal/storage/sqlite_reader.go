package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/roman-kulish/radio-surveillance/internal/spectrum"
	"github.com/roman-kulish/radio-surveillance/internal/telemetry"
)

// ErrNoData indicates either that no spectrum data exists for the given parameters,
// or that all available data has been read from the spectrum reader.
var ErrNoData = fmt.Errorf("no data available")

// SpectralData is a constraint for types that can represent spectrum measurements,
// either basic spectral points or those enriched with telemetry data.
type SpectralData interface {
	spectrum.SpectralPoint | spectrum.SpectralPointWithTelemetry

	GetFrequency() float64
	GetBinWidth() float64
	GetNumSamples() int
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
type ReaderOption[T SpectralData] func(*SqliteSpectrumReader[T])

// WithMinFreq sets the minimum frequency filter for the spectrum reader.
// Spectrum points with frequencies below this value will be excluded.
func WithMinFreq[T SpectralData](f float64) ReaderOption[T] {
	return func(r *SqliteSpectrumReader[T]) {
		r.minFreq = &f
	}
}

// WithMaxFreq sets the maximum frequency filter for the spectrum reader.
// Spectrum points with frequencies above this value will be excluded.
func WithMaxFreq[T SpectralData](f float64) ReaderOption[T] {
	return func(r *SqliteSpectrumReader[T]) {
		r.maxFreq = &f
	}
}

// WithFreqRange sets both minimum and maximum frequency filters.
// This is a convenience function equivalent to applying both WithMinFreq
// and WithMaxFreq.
func WithFreqRange[T SpectralData](minFreq, maxFreq float64) ReaderOption[T] {
	return func(r *SqliteSpectrumReader[T]) {
		r.minFreq = &minFreq
		r.maxFreq = &maxFreq
	}
}

// WithStartTime sets the start time filter for the spectrum reader.
// Spectrum points with timestamps before this time will be excluded.
func WithStartTime[T SpectralData](t time.Time) ReaderOption[T] {
	return func(r *SqliteSpectrumReader[T]) {
		r.startTime = &t
	}
}

// WithEndTime sets the end time filter for the spectrum reader.
// Spectrum points with timestamps after this time will be excluded.
func WithEndTime[T SpectralData](t time.Time) ReaderOption[T] {
	return func(r *SqliteSpectrumReader[T]) {
		r.endTime = &t
	}
}

// WithTimeRange sets both start and end time filters.
// This is a convenience function equivalent to applying both WithStartTime
// and WithEndTime.
func WithTimeRange[T SpectralData](startTime, endTime time.Time) ReaderOption[T] {
	return func(r *SqliteSpectrumReader[T]) {
		r.startTime = &startTime
		r.endTime = &endTime
	}
}

// newSqliteSpectrumReader creates a new SpectrumReader instance for reading spectral data from a database,
// applying optional filters.
func newSqliteSpectrumReader[T SpectralData](db *sql.DB, sessionID int64, includeTelemetry bool, opts ...ReaderOption[T],
) (*SqliteSpectrumReader[T], error) {
	sr := &SqliteSpectrumReader[T]{
		db:               db,
		sessionID:        sessionID,
		includeTelemetry: includeTelemetry,
	}
	for _, opt := range opts {
		opt(sr)
	}
	if err := sr.init(context.Background()); err != nil {
		return nil, fmt.Errorf("initializing reader: %w", err)
	}
	return sr, nil
}

// SqliteSpectrumReader implements SpectrumReader for SQLite database backend.
type SqliteSpectrumReader[T SpectralData] struct {
	db *sql.DB

	sessionID        int64
	session          *spectrum.ScanSession
	includeTelemetry bool
	numChunks        int

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

func (sr *SqliteSpectrumReader[T]) init(ctx context.Context) error {
	if sr.db == nil {
		return errors.New("database connection required")
	}
	if sr.sessionID <= 0 {
		return errors.New("session ID required")
	}

	steps := []struct {
		msg string
		fn  func(context.Context) error
	}{
		{msg: "loading session", fn: sr.loadSession},
		{msg: "initializing filters", fn: sr.initFilters},
		{msg: "initializing query", fn: sr.initQuery},
	}
	for _, s := range steps {
		if err := s.fn(ctx); err != nil {
			return fmt.Errorf("%s: %w", s.msg, err)
		}
	}
	return nil
}

func (sr *SqliteSpectrumReader[T]) loadSession(ctx context.Context) (err error) {
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

func (sr *SqliteSpectrumReader[T]) initFilters(ctx context.Context) (err error) {
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
	var startTime, endTime buggySqliteDatetime
	if err = stmt.QueryRowContext(ctx, sr.sessionID).Scan(&minFreq, &maxFreq, &startTime, &endTime); err != nil {
		return fmt.Errorf("scanning filters data: %w", err)
	}

	if sr.minFreq == nil {
		sr.minFreq = &minFreq
	}
	if sr.maxFreq == nil {
		sr.maxFreq = &maxFreq
	}
	if sr.startTime == nil {
		sr.startTime = &startTime.Datetime
	}
	if sr.endTime == nil {
		sr.endTime = &endTime.Datetime
	}

	return nil
}

func (sr *SqliteSpectrumReader[T]) initQuery(ctx context.Context) (err error) {
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

func (sr *SqliteSpectrumReader[T]) convertPoint(point any) (T, error) {
	result, ok := point.(T)
	if !ok {
		var zero T
		return zero, fmt.Errorf("invalid type conversion from %T to %T", point, zero)
	}
	return result, nil
}

func (sr *SqliteSpectrumReader[T]) scanSample() (time.Time, T, error) {
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

	result, err := sr.convertPoint(point)
	return timestamp, result, err
}

func (sr *SqliteSpectrumReader[T]) scanSampleWithTelemetry() (time.Time, T, error) {
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
		result, err := sr.convertPoint(point)
		return timestamp, result, err
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

	result, err := sr.convertPoint(point)
	return timestamp, result, err
}

func (sr *SqliteSpectrumReader[T]) createZeroPoint(freq float64, template T) T {
	zeroPower := 0.0
	switch v := any(template).(type) {
	case spectrum.SpectralPointWithTelemetry:
		point := spectrum.SpectralPointWithTelemetry{
			SpectralPoint: spectrum.SpectralPoint{
				Frequency:  freq,
				Power:      &zeroPower,
				BinWidth:   template.GetBinWidth(),
				NumSamples: template.GetNumSamples(),
			},
			Telemetry: v.Telemetry,
		}
		return any(point).(T)

	default:
		point := spectrum.SpectralPoint{
			Frequency:  freq,
			Power:      &zeroPower,
			BinWidth:   template.GetBinWidth(),
			NumSamples: template.GetNumSamples(),
		}
		return any(point).(T)
	}
}

// fillFrequencyRange fills a slice with zero power spectral points for the given frequency range.
// Power readings can be dropped, not properly aligned or first/last data points can be selected
// in the middle of the spectrum. We can either do (1) some sophisticated queries to try and select
// complete data, if possible or (2) drop incomplete spans, or (3) fill the gaps with zero power
// points. The latter is the simplest possible approach.
func (sr *SqliteSpectrumReader[T]) fillFrequencyRange(start, end float64, template T) ([]T, error) {
	binWidth := template.GetBinWidth()
	if binWidth <= 0 {
		return nil, fmt.Errorf("invalid bin width: %f", binWidth)
	}

	numPoints := int(math.Floor((end-start)/binWidth)) + 1 // add extra step
	if numPoints <= 0 {
		return nil, nil
	}

	points := make([]T, 0, numPoints)
	for i := 0; i < numPoints; i++ {
		freq := start + float64(i)*binWidth
		if freq <= end { // make sure there is no overlap
			points = append(points, sr.createZeroPoint(freq, template))
			continue
		}
		break
	}
	return points, nil
}

func (sr *SqliteSpectrumReader[T]) Session() *spectrum.ScanSession {
	return sr.session
}

func (sr *SqliteSpectrumReader[T]) Next(ctx context.Context) bool {
	if sr.err != nil || sr.rows == nil {
		return false
	}

	if sr.nextSampleExists {
		if sr.numChunks == 0 {
			n := (*sr.maxFreq - *sr.minFreq) / sr.nextSample.GetBinWidth()
			sr.numChunks = int(n * 1.1) // add 10% to account for rounding errors and variations in bin width
		}
		sr.currentSpan = &spectrum.SpectralSpan[T]{
			Timestamp:      sr.nextSpanStartTimestamp,
			FrequencyStart: sr.nextSample.GetFrequency(),
			Samples:        make([]T, 0, sr.numChunks),
		}
		sr.currentSpan.Samples = append(sr.currentSpan.Samples, sr.nextSample)
		sr.nextSampleExists = false

		// Detect and fill gaps between the beginning of the spectrum and sr.nextSample.Frequency
		if freqGreater(sr.nextSample.GetFrequency(), *sr.minFreq, sr.nextSample.GetBinWidth()) {
			gapPoints, err := sr.fillFrequencyRange(*sr.minFreq, sr.nextSample.GetFrequency(), sr.nextSample)
			if err != nil {
				sr.err = fmt.Errorf("filling min frequency gap: %w", err)
				return false
			}
			sr.currentSpan.Samples = append(gapPoints, sr.currentSpan.Samples...)
			sr.currentSpan.FrequencyStart = *sr.minFreq
		}
	}

	for {
		select {
		case <-ctx.Done():
			sr.err = ctx.Err()
			return false
		default:
		}

		if !sr.rows.Next() {
			if sr.currentSpan != nil && len(sr.currentSpan.Samples) > 0 {
				lastSample := sr.currentSpan.Samples[len(sr.currentSpan.Samples)-1]
				sr.currentSpan.FrequencyEnd = lastSample.GetFrequency()

				// Detect and fill gaps between the last reading and the end of the spectrum
				if freqLess(lastSample.GetFrequency(), *sr.maxFreq, lastSample.GetBinWidth()) {
					gapPoints, err := sr.fillFrequencyRange(lastSample.GetFrequency()+lastSample.GetBinWidth(), *sr.maxFreq, lastSample)
					if err != nil {
						sr.err = fmt.Errorf("filling max frequency gap: %w", err)
						return false
					}
					sr.currentSpan.Samples = append(sr.currentSpan.Samples, gapPoints...)
					sr.currentSpan.FrequencyEnd = *sr.maxFreq
				}

				sr.err = ErrNoData
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
			if sr.numChunks == 0 {
				n := (*sr.maxFreq - *sr.minFreq) / sample.GetBinWidth()
				sr.numChunks = int(n * 1.1)
			}
			sr.currentSpan = &spectrum.SpectralSpan[T]{
				Timestamp:      timestamp,
				FrequencyStart: sample.GetFrequency(),
				Samples:        make([]T, 0, sr.numChunks),
			}
			sr.currentSpan.Samples = append(sr.currentSpan.Samples, sample)

			// Detect and fill the gap between the beginning of the spectrum and sr.nextSample.Frequency
			if freqGreater(sample.GetFrequency(), *sr.minFreq, sample.GetBinWidth()) {
				gapPoints, err := sr.fillFrequencyRange(*sr.minFreq, sample.GetFrequency(), sample)
				if err != nil {
					sr.err = fmt.Errorf("filling min frequency gap: %w", err)
					return false
				}
				sr.currentSpan.Samples = append(gapPoints, sr.currentSpan.Samples...)
				sr.currentSpan.FrequencyStart = *sr.minFreq
			}
			continue
		}

		// Check for frequency rollover only
		lastSample := sr.currentSpan.Samples[len(sr.currentSpan.Samples)-1]
		if sample.GetFrequency() < lastSample.GetFrequency() {
			// Frequency rolled over - complete current span
			sr.currentSpan.FrequencyEnd = lastSample.GetFrequency()

			// Detect and fill the gap between the last reading and the end of the spectrum
			if freqLess(lastSample.GetFrequency(), *sr.maxFreq, lastSample.GetBinWidth()) {
				gapPoints, err := sr.fillFrequencyRange(lastSample.GetFrequency()+lastSample.GetBinWidth(), *sr.maxFreq, lastSample)
				if err != nil {
					sr.err = fmt.Errorf("filling max frequency gap: %w", err)
					return false
				}
				sr.currentSpan.Samples = append(sr.currentSpan.Samples, gapPoints...)
				sr.currentSpan.FrequencyEnd = *sr.maxFreq
			}

			sr.nextSample = sample
			sr.nextSampleExists = true
			sr.nextSpanStartTimestamp = timestamp
			return true
		}

		// Detect and fill the gap between two data points
		if freqLess(lastSample.GetFrequency()+lastSample.GetBinWidth(), sample.GetFrequency(), lastSample.GetBinWidth()) {
			gapPoints, err := sr.fillFrequencyRange(lastSample.GetFrequency()+lastSample.GetBinWidth(), sample.GetFrequency(), lastSample)
			if err != nil {
				sr.err = fmt.Errorf("filling frequency gap between data points: %w", err)
				return false
			}
			sr.currentSpan.Samples = append(sr.currentSpan.Samples, gapPoints...)
		}

		// Add sample to current span
		sr.currentSpan.Samples = append(sr.currentSpan.Samples, sample)
	}
}

func (sr *SqliteSpectrumReader[T]) Current() *spectrum.SpectralSpan[T] {
	return sr.currentSpan
}

func (sr *SqliteSpectrumReader[T]) Error() error {
	if sr.err != nil && !errors.Is(sr.err, ErrNoData) {
		return sr.err
	}
	if sr.rows != nil {
		return sr.rows.Err()
	}
	return nil
}

func (sr *SqliteSpectrumReader[T]) Close() error {
	if sr.rows != nil {
		err := sr.rows.Close()
		sr.currentSpan = nil
		sr.nextSampleExists = false
		sr.rows = nil
		return err
	}
	return nil
}
