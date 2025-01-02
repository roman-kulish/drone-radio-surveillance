package storage

import (
	_ "embed"
)

const (
	insertSessionSQL = `
INSERT INTO sessions (
                      start_time, 
                      device_type, 
                      device_id, 
                      config) 
VALUES (CURRENT_TIMESTAMP, ?, ?, ?)`

	selectSessionSQL = `
SELECT 
    id, 
    start_time, 
    device_type, 
    device_id, 
    config 
FROM sessions 
WHERE 
    id = ?`

	selectSessionsSQL = `
SELECT 
    id, 
    start_time, 
    device_type, 
    device_id, 
    config 
FROM sessions`

	insertTelemetrySQL = `
INSERT INTO telemetry (session_id,
                       timestamp,
                       latitude,
                       longitude,
                       altitude,
                       roll,
                       pitch, yaw,
                       accel_x,
                       accel_y,
                       accel_z,
                       radio_rssi)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	insertSampleSQL = `
INSERT INTO samples (session_id,
                     timestamp,
                     frequency,
                     bin_width,
                     power,
                     num_samples,
                     telemetry_id)
VALUES (?, ?, ?, ?, ?, ?, ?)`
)

//go:embed schema.sql
var schemaSQL string
