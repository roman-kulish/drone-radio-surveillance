package storage

import (
	_ "embed"
)

//go:embed init_schema.sql
var initSchemaSQL string

//go:embed init_indexes.sql
var initIndexesSQL string

const (
	// insertSessionSQL creates a new capture session record.
	// Parameters:
	//   1. device_type (string): Type of SDR device (e.g., 'rtl-sdr', 'hackrf')
	//   2. device_id (string): Unique identifier of the device
	//   3. config (string|null): Optional JSON configuration
	// Returns: last inserted ID
	insertSessionSQL = `
        INSERT INTO sessions (
            start_time,
            device_type,
            device_id,
            config
        ) 
        VALUES (CURRENT_TIMESTAMP, ?, ?, ?)`

	// selectSessionSQL retrieves a single session by ID.
	// Parameters:
	//   1. id (int64): Session identifier
	// Returns: Full session record
	selectSessionSQL = `
        SELECT 
            id,
            start_time,
            device_type,
            device_id,
            config
        FROM sessions 
        WHERE id = ?`

	// selectSessionsSQL retrieves all capture sessions.
	// Returns: All session records
	selectSessionsSQL = `
        SELECT 
            id,
            start_time,
            device_type,
            device_id,
            config
        FROM sessions`

	// insertTelemetrySQL stores drone telemetry data.
	// Parameters:
	//   1. session_id (int64): Associated session ID
	//   2. timestamp (datetime): Time of telemetry measurement
	//   3-12. Various telemetry values
	// Returns: last inserted ID
	insertTelemetrySQL = `
        INSERT INTO telemetry (
            session_id,
            timestamp,
            latitude,
            longitude,
            altitude,
            roll,
            pitch,
            yaw,
            accel_x,
            accel_y,
            accel_z,
            radio_rssi
        )
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	// selectFilterValuesSQL retrieves the bounds of frequency and time
	// for all samples in a given session.
	// Parameters:
	//   1. session_id (int64): Session to analyze
	// Returns: min/max frequency and timestamp values
	selectFilterValuesSQL = `
	    SELECT
	        MIN(frequency),
	        MAX(frequency),
	        MIN(timestamp),
	        MAX(timestamp)
	    FROM samples
	    WHERE session_id = ?`

	// selectSamplesSQL retrieves spectrum samples within specified time and frequency bounds.
	// Parameters:
	//   1. session_id (int64): Session to query
	//   2. start_time (datetime): Start of time window
	//   3. end_time (datetime): End of time window
	//   4. min_freq (float64): Lower frequency bound in Hz
	//   5. max_freq (float64): Upper frequency bound in Hz
	// Returns: Ordered spectrum samples
	// Required indexes:
	//   - samples(session_id, timestamp, frequency)
	//   - samples(session_id, frequency, timestamp)
	selectSamplesSQL = `
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
		ORDER BY timestamp, frequency`

	// selectSamplesWithTelemetrySQL retrieves spectrum samples enriched with telemetry data
	// using the v_samples_with_telemetry view that joins samples with telemetry.
	// Parameters:
	//   1. session_id (int64): Session to query
	//   2. start_time (datetime): Start of time window
	//   3. end_time (datetime): End of time window
	//   4. min_freq (float64): Lower frequency bound in Hz
	//   5. max_freq (float64): Upper frequency bound in Hz
	// Returns: Ordered spectrum samples with synchronized telemetry data
	// Required indexes:
	//   - samples(session_id, timestamp, frequency)
	//   - telemetry(session_id, timestamp)
	selectSamplesWithTelemetrySQL = `
		SELECT
		    timestamp,
		    frequency,
		    power,
		    bin_width,
		    num_samples,
		    telemetry_id,
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
		ORDER BY timestamp, frequency`
)
