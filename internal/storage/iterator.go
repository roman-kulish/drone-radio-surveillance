package storage

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func WithMinFreq[T Sample | SampleWithTelemetry](minFreq float64) func(*SpanIterator[T]) {
	return func(i *SpanIterator[T]) {
		i.minFreq = &minFreq
	}
}

func WithMaxFreq[T Sample | SampleWithTelemetry](maxFreq float64) func(*SpanIterator[T]) {
	return func(i *SpanIterator[T]) {
		i.maxFreq = &maxFreq
	}
}

func withMinMaxFreq[T Sample | SampleWithTelemetry](minFreq, maxFreq float64) func(*SpanIterator[T]) {
	return func(i *SpanIterator[T]) {
		i.minFreq = &minFreq
		i.maxFreq = &maxFreq
	}
}

func WithStartTime[T Sample | SampleWithTelemetry](startTime time.Time) func(*SpanIterator[T]) {
	return func(i *SpanIterator[T]) {
		i.startTime = &startTime
	}
}

func WithEndTime[T Sample | SampleWithTelemetry](endTime time.Time) func(*SpanIterator[T]) {
	return func(i *SpanIterator[T]) {
		i.endTime = &endTime
	}
}

func WithTimeRange[T Sample | SampleWithTelemetry](startTime, endTime time.Time) func(*SpanIterator[T]) {
	return func(i *SpanIterator[T]) {
		i.startTime = &startTime
		i.endTime = &endTime
	}
}

// SpanIterator provides buffered iteration over frequency spans
type SpanIterator[T Sample | SampleWithTelemetry] struct {
	db               *sql.DB
	sessionID        int64
	session          Session
	includeTelemetry bool
	startTime        *time.Time
	endTime          *time.Time
	minFreq          *float64
	maxFreq          *float64

	currentSpan *FrequencySpan[T]
	nextSpan    *FrequencySpan[T]
	rows        *sql.Rows
	err         error
}

// Next advances to the next frequency span
func (si *SpanIterator[T]) Next() bool {
	var currentTimestamp time.Time
	var samples []Sample
	var firstSample bool = true
	var startFreq float64

	for si.rows.Next() {
		var timestamp time.Time
		var freq, power, binWidth float64

		if err := i.rows.Scan(&timestamp, &freq, &power, &binWidth); err != nil {
			i.err = err
			return false
		}

		// Handle first sample
		if firstSample {
			currentTimestamp = timestamp
			startFreq = freq
			firstSample = false
		}

		// If timestamp changed, buffer the span and start a new one
		if timestamp != currentTimestamp {
			// Store completed span
			span := &FrequencySpan{
				Timestamp:  currentTimestamp,
				StartFreq:  startFreq,
				EndFreq:    freq,
				Samples:    samples,
				IsComplete: true,
			}

			// If this is first span, make it current
			if len(i.buffer) == 0 {
				i.currentSpan = span
			} else {
				i.buffer = append(i.buffer, span)
			}

			// Start new span
			samples = []Sample{}
			currentTimestamp = timestamp
			startFreq = freq
		}

		// Add sample to current collection
		samples = append(samples, Sample{
			Frequency: freq,
			Power:     power,
			BinWidth:  binWidth,
		})
	}

	// Handle any remaining samples
	if len(samples) > 0 {
		span := &FrequencySpan{
			Timestamp:  currentTimestamp,
			StartFreq:  startFreq,
			EndFreq:    samples[len(samples)-1].Frequency,
			Samples:    samples,
			IsComplete: true,
		}

		if i.currentSpan == nil {
			i.currentSpan = span
		} else {
			i.buffer = append(i.buffer, span)
		}
	}

	return i.currentSpan != nil
}

// Current returns the current frequency span
func (si *SpanIterator[T]) Current() *FrequencySpan[T] {
	return si.currentSpan
}

// Error returns any error that occurred during iteration
func (si *SpanIterator[T]) Error() error {
	if si.err != nil {
		return si.err
	}
	return si.rows.Err()
}

// Close releases the database resources
func (si *SpanIterator[T]) Close() error {
	return si.rows.Close()
}

func (si *SpanIterator[T]) init() error {
	if err := si.initSession(); err != nil {
		return fmt.Errorf("loading session data: %w", err)
	}
	if err := si.initFilters(); err != nil {
		return fmt.Errorf("setting up filters: %w", err)
	}
	if err := si.initQuery(); err != nil {
		return fmt.Errorf("setting up query: %w", err)
	}
	return nil
}

func (si *SpanIterator[T]) initSession() error {
	var session sessionData
	stmt, err := si.db.Prepare(selectSessionSQL)
	if err != nil {
		return err
	}
	defer stmt.Close()

	if err = stmt.QueryRow(si.sessionID).Scan(&session.StartTime, &session.DeviceType, &session.DeviceID, &session.Config); err != nil {
		return err
	}

	var config *string
	if session.Config.Valid {
		config = &session.Config.String
	}

	si.session = Session{
		ID:         si.sessionID,
		StartTime:  session.StartTime,
		DeviceType: session.DeviceType,
		DeviceID:   session.DeviceID,
		Config:     config,
	}
	return nil
}

const selectFilterValuesSQL = `
SELECT 
    MIN(frequency), 
    MAX(frequency), 
    MIN(timestamp), 
    MAX(timestamp)
FROM samples
WHERE session_id = ?
`

func (si *SpanIterator[T]) initFilters() error {
	if si.minFreq != nil && si.maxFreq != nil && si.startTime != nil && si.endTime != nil {
		return nil
	}

	var minFreq, maxFreq float64
	var startTime, endTime time.Time

	// If frequency range not specified, get min/max from database.
	// If time range not specified, get min/max from database.
	stmt, err := si.db.Prepare(selectFilterValuesSQL)
	if err != nil {
		return err
	}
	defer stmt.Close()

	if err = stmt.QueryRow(si.sessionID).Scan(&minFreq, &maxFreq, &startTime, &endTime); err != nil {
		return err
	}

	if si.minFreq == nil {
		si.minFreq = &minFreq
	}
	if si.maxFreq == nil {
		si.maxFreq = &maxFreq
	}
	if si.startTime == nil {
		si.startTime = &startTime
	}
	if si.endTime == nil {
		si.endTime = &endTime
	}

	return nil
}

const selectSamplesSQL = `
SELECT 
    timestamp, 
    frequency, 
    power, 
    bin_width, 
    num_samples
FROM samples
WHERE 
    session_id = ?
	AND timestamp BETWEEN ? AND ?
  	AND frequency BETWEEN ? AND ?
ORDER BY timestamp, frequency
`

const selectSamplesWithTelemetrySQL = `
SELECT 
    timestamp, 
    frequency, 
    power, 
    bin_width, 
    num_samples,
    latitude,
    longitude,
    altitude,
    roll,
    pitch,
    yaw,
    accel_x,
    accel_y,
    accel_z,
    ground_speed,
    ground_course,
    radio_rssi
FROM v_samples_with_telemetry
WHERE 
    session_id = ?
  	AND timestamp BETWEEN ? AND ?
  	AND frequency BETWEEN ? AND ?
ORDER BY timestamp, frequency
`

func (si *SpanIterator[T]) initQuery() error {
	// Query ordered by timestamp and frequency to ensure proper span building
	query := selectSamplesSQL

	if si.includeTelemetry {
		query = selectSamplesWithTelemetrySQL
	}

	stmt, err := si.db.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	rows, err := stmt.Query(si.sessionID, si.startTime, si.endTime, si.minFreq, si.maxFreq)
	if err != nil {
		return err
	}

	si.rows = rows
	return nil
}

func (si *SpanIterator[T]) scanSample() (time.Time, Sample, error) {
	var sample sampleData
	if err := si.rows.Scan(&sample.Timestamp, &sample.Frequency, &sample.Power, &sample.BinWidth, &sample.NumSamples); err != nil {
		return time.Time{}, Sample{}, err
	}

	var power *float64
	if sample.Power.Valid {
		power = &sample.Power.Float64
	}

	s := Sample{
		Frequency:  sample.Frequency,
		Power:      power,
		BinWidth:   sample.BinWidth,
		NumSamples: sample.NumSamples,
	}

	return sample.Timestamp, s, nil
}

func (si *SpanIterator[T]) scanSampleWithTelemetry() (SampleWithTelemetry, error) {

}

// Iterator provides access to the samples data
type Iterator struct {
	db *sql.DB
}

// NewIterator initializes and returns a new Iterator instance using the specified database path
func NewIterator(dbPath string) (*Iterator, error) {
	db, err := sql.Open("sqlite3?mode=ro", dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	return &Iterator{db}, nil
}

// Samples retrieves an iterator over frequency samples for the given session ID using optional configuration functions
func (i Iterator) Samples(sessionID int64, options ...func(*SpanIterator[Sample])) (*SpanIterator[Sample], error) {
	iter := &SpanIterator[Sample]{
		db:        i.db,
		sessionID: sessionID,
	}
	for _, option := range options {
		option(iter)
	}

	return iter, iter.init()
}

// SamplesWithTelemetry retrieves an iterator over frequency samples with associated telemetry for the given session ID
func (i Iterator) SamplesWithTelemetry(sessionID int64, options ...func(*SpanIterator[SampleWithTelemetry])) (*SpanIterator[SampleWithTelemetry], error) {
	iter := &SpanIterator[SampleWithTelemetry]{
		db:               i.db,
		sessionID:        sessionID,
		includeTelemetry: true,
	}
	for _, option := range options {
		option(iter)
	}

	return iter, iter.init()
}

// Close releases any resources held by the Iterator, closing the underlying database connection
func (i Iterator) Close() error {
	return i.db.Close()
}
