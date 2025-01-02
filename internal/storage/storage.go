package storage

import (
	"database/sql"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	_ "github.com/mattn/go-sqlite3"
)

//go:embed schema.sql
var schemaSQL string

// SessionData represents a data collection session

// Store handles database operations
type Store struct {
	dbPath string

	writeDB     *sql.DB
	writeDBOnce sync.Once
	writeDBErr  error

	readDB     *sql.DB
	readDBOnce sync.Once
	readDBErr  error

	closeOnce sync.Once
	closeErr  error
}

// New creates a new database connection and initializes the schema
func New(dbPath string) (*Store, error) {
	return &Store{dbPath: dbPath}, nil
}

func initSchema(db *sql.DB) error {
	_, err := db.Exec(schemaSQL)
	return err
}

func (s *Store) getWriteDB() (*sql.DB, error) {
	s.writeDBOnce.Do(func() {
		db, err := sql.Open("sqlite3?_journal_mode=WAL&_synchronous=NORMAL", s.dbPath)
		if err != nil {
			s.writeDBErr = err
			return
		}

		if err = initSchema(db); err != nil {
			_ = db.Close()
			s.writeDBErr = err
			return
		}

		s.writeDB = db
	})

	return s.writeDB, s.writeDBErr
}

func (s *Store) getReadDB() (*sql.DB, error) {
	s.readDBOnce.Do(func() {
		db, err := sql.Open("sqlite3?mode=ro", s.dbPath)
		if err != nil {
			s.readDBErr = err
			return
		}
		s.readDB = db
	})

	return s.readDB, s.readDBErr
}

const insertSessionSQL = `
INSERT INTO sessions (start_time, device_type, device_id, config) 
VALUES (CURRENT_TIMESTAMP, ?, ?, ?)`

// CreateSession creates a new session and returns its ID
func (s *Store) CreateSession(deviceType, deviceID string, config any) (sessionID int64, err error) {
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
			var p []byte
			if p, err = json.Marshal(config); err != nil {
				err = fmt.Errorf("marshaling config: %w", err)
				return
			}

			configData.Valid = true
			configData.String = string(p)
		}
	}

	db, err := s.getWriteDB()
	if err != nil {
		err = fmt.Errorf("getting write connection: %w", err)
		return
	}

	stmt, err := db.Prepare(insertSessionSQL)
	if err != nil {
		err = fmt.Errorf("preparing statement: %w", err)
		return
	}
	defer func() {
		if cErr := stmt.Close(); cErr != nil && err == nil {
			err = fmt.Errorf("closing statement: %w", cErr)
		}
	}()

	result, err := stmt.Exec(deviceType, deviceID, configData)
	if err != nil {
		err = fmt.Errorf("inserting session: %w", err)
		return
	}

	return result.LastInsertId()
}

const selectSessionSQL = `
SELECT 
    id, 
    start_time, 
    device_type, 
    device_id, 
    config 
FROM sessions 
WHERE 
    id = ?`

// Session returns a session by its ID
func (s *Store) Session(id int64) (session *SessionData, err error) {
	db, err := s.getReadDB()
	if err != nil {
		err = fmt.Errorf("getting read connection: %w", err)
		return
	}

	stmt, err := db.Prepare(selectSessionSQL)
	if err != nil {
		err = fmt.Errorf("preparing statement: %w", err)
		return
	}
	defer func() {
		if cErr := stmt.Close(); cErr != nil && err == nil {
			err = fmt.Errorf("closing statement: %w", cErr)
		}
	}()

	var sess SessionData
	if err = stmt.QueryRow(id).Scan(&sess.ID, &sess.StartTime, &sess.DeviceType, &sess.DeviceID, &sess.Config); err != nil {
		err = fmt.Errorf("scanning session: %w", err)
		return
	}
	return &sess, nil
}

const selectSessionsSQL = `
SELECT 
    id, 
    start_time, 
    device_type, 
    device_id, 
    config 
FROM sessions
`

func (s *Store) Sessions() (sessions []SessionData, err error) {
	db, err := s.getReadDB()
	if err != nil {
		err = fmt.Errorf("getting read connection: %w", err)
		return
	}

	rows, err := db.Query(selectSessionsSQL)
	if err != nil {
		err = fmt.Errorf("querying sessions: %w", err)
		return
	}
	defer func() {
		if cErr := rows.Close(); cErr != nil && err == nil {
			err = fmt.Errorf("closing rows: %w", cErr)
		}
	}()

	for rows.Next() {
		var sess SessionData
		if err = rows.Scan(&sess.ID, &sess.StartTime, &sess.DeviceType, &sess.DeviceID, &sess.Config); err != nil {
			err = fmt.Errorf("scanning session: %w", err)
			return
		}
		sessions = append(sessions, sess)
	}
	return
}

const insertTelemetrySQL = `
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
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`

// InsertTelemetry inserts telemetry data and returns its ID
func (s *Store) InsertTelemetry(t TelemetryData) (telemetryID int64, err error) {
	db, err := s.getWriteDB()
	if err != nil {
		err = fmt.Errorf("getting write connection: %w", err)
		return
	}

	stmt, err := db.Prepare(insertTelemetrySQL)
	if err != nil {
		err = fmt.Errorf("preparing statement: %w", err)
		return
	}
	defer func() {
		if cErr := stmt.Close(); cErr != nil && err == nil {
			err = fmt.Errorf("closing statement: %w", cErr)
		}
	}()

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
		err = fmt.Errorf("inserting telemetry: %w", err)
		return
	}

	return result.LastInsertId()
}

const insertSampleSQL = `
INSERT INTO samples (session_id,
                     timestamp,
                     frequency,
                     bin_width,
                     power,
                     num_samples,
                     telemetry_id)
VALUES (?, ?, ?, ?, ?, ?, ?)
`

// BatchInsertSamples inserts multiple samples in a single transaction
func (s *Store) BatchInsertSamples(samples []SampleData) (err error) {
	if len(samples) == 0 {
		return
	}

	db, err := s.getWriteDB()
	if err != nil {
		return fmt.Errorf("getting write connection: %w", err)
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() {
		if cErr := tx.Rollback(); cErr != nil && err == nil {
			err = fmt.Errorf("rolling back transaction: %w", cErr)
		}
	}()

	stmt, err := tx.Prepare(insertSampleSQL)
	if err != nil {
		return fmt.Errorf("preparing statement: %w", err)
	}
	defer func() {
		if cErr := stmt.Close(); cErr != nil && err == nil {
			err = fmt.Errorf("closing statement: %w", cErr)
		}
	}()

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

	return
}

// Close closes the database connection
func (s *Store) Close() error {
	s.closeOnce.Do(func() {
		var writeErr, readErr error

		if s.writeDB != nil {
			writeErr = s.writeDB.Close()
			s.writeDB = nil
		}

		if s.readDB != nil {
			readErr = s.readDB.Close()
			s.readDB = nil
		}

		switch {
		case writeErr != nil && readErr != nil:
			s.closeErr = errors.Join(writeErr, readErr)
		case writeErr != nil:
			s.closeErr = writeErr
		case readErr != nil:
			s.closeErr = readErr
		}
	})

	return s.closeErr
}
