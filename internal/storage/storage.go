package storage

import (
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

//go:embed schema.sql
var schemaSQL string

// SessionData represents a data collection session

// Store handles database operations
type Store struct {
	db *sql.DB
}

// New creates a new database connection and initializes the schema
func New(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite3?_journal_mode=WAL&_synchronous=NORMAL", dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	if err = initSchema(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("initializing schema: %w", err)
	}

	return &Store{db}, nil
}

// Close closes the database connection
func (s *Store) Close() error {
	return s.db.Close()
}

// CreateSession creates a new session and returns its ID
func (s *Store) CreateSession(deviceType, deviceID string, config any) (int64, error) {
	var configData sql.NullString

	if config != nil {
		switch config.(type) {
		case string:
			configData.Valid = true
			configData.String = config.(string)

		case []byte:
			configData.Valid = true
			configData.String = string(config.([]byte))

		default:
			v, err := json.Marshal(config)
			if err != nil {
				return 0, fmt.Errorf("marshaling config: %w", err)
			}

			configData.Valid = true
			configData.String = string(v)
		}
	}

	stmt, err := s.db.Prepare(`INSERT INTO sessions (start_time, device_type, device_id, config_json) VALUES (CURRENT_TIMESTAMP, ?, ?, ?)`)
	if err != nil {
		return 0, fmt.Errorf("preparing statement: %w", err)
	}
	defer stmt.Close()

	result, err := stmt.Exec(deviceType, deviceID, configData)
	if err != nil {
		return 0, fmt.Errorf("inserting session: %w", err)
	}

	return result.LastInsertId()
}

// Session returns a session by its ID
func (s *Store) Session(id int64) (*SessionData, error) {
	stmt, err := s.db.Prepare(`SELECT id, start_time, device_type, device_id, config FROM sessions WHERE id = ?`)
	if err != nil {
		return nil, fmt.Errorf("preparing statement: %w", err)
	}
	defer stmt.Close()

	var session SessionData
	if err = stmt.QueryRow(id).Scan(&session.ID, &session.StartTime, &session.DeviceType, &session.DeviceID, &session.Config); err != nil {
		return nil, fmt.Errorf("querying session: %w", err)
	}

	return &session, nil
}

func (s *Store) Sessions() ([]SessionData, error) {
	rows, err := s.db.Query(`SELECT id, start_time, device_type, device_id, config FROM sessions`)
	if err != nil {
		return nil, fmt.Errorf("querying sessions: %w", err)
	}
	defer rows.Close()

	var sessions []SessionData
	for rows.Next() {
		var session SessionData
		if err = rows.Scan(&session.ID, &session.StartTime, &session.DeviceType, &session.DeviceID, &session.Config); err != nil {
			return nil, fmt.Errorf("scanning session: %w", err)
		}
		sessions = append(sessions, session)
	}

	return sessions, nil
}

// InsertTelemetry inserts telemetry data and returns its ID
func (s *Store) InsertTelemetry(t TelemetryData) (int64, error) {
	stmt, err := s.db.Prepare(`INSERT INTO telemetry (session_id,
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
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, fmt.Errorf("preparing statement: %w", err)
	}
	defer stmt.Close()

	result, err := stmt.Exec(
		t.SessionID,
		t.Timestamp,
		t.Latitude,
		t.Longitude,
		t.Altitude,
		t.Roll,
		t.Pitch,
		t.Yaw,
		t.AccelX,
		t.AccelY,
		t.AccelZ,
		t.GroundSpeed,
		t.GroundCourse,
		t.RadioRSSI,
	)
	if err != nil {
		return 0, fmt.Errorf("inserting telemetry: %w", err)
	}

	return result.LastInsertId()
}

// BatchInsertSamples inserts multiple samples in a single transaction
func (s *Store) BatchInsertSamples(samples []SampleData) error {
	if len(samples) == 0 {
		return nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT INTO samples (session_id,
                     timestamp,
                     frequency,
                     bin_width,
                     power,
                     num_samples,
                     telemetry_id)
VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("preparing statement: %w", err)
	}
	defer stmt.Close()

	for _, sample := range samples {
		_, err = stmt.Exec(
			sample.SessionID,
			sample.Timestamp,
			sample.Frequency,
			sample.BinWidth,
			sample.Power,
			sample.NumSamples,
			sample.TelemetryID,
		)
		if err != nil {
			return fmt.Errorf("inserting sample: %w", err)
		}
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	return nil
}

// Internal function to initialize database schema
func initSchema(db *sql.DB) error {
	_, err := db.Exec(schemaSQL)
	return err
}
