package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"

	_ "github.com/mattn/go-sqlite3"
	"github.com/roman-kulish/radio-surveillance/internal/sdr"
	"github.com/roman-kulish/radio-surveillance/internal/spectrum"
	"github.com/roman-kulish/radio-surveillance/internal/telemetry"
)

// Store provides an interface for managing radio surveillance data storage operations.
// It handles sessions, telemetry data, and spectrum sweep results in a thread-safe manner.
// All operations that write to the database should be considered atomic.
type Store interface {
	// CreateSession initializes a new scanning session and returns its unique identifier.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeouts
	//   - deviceType: Type of SDR device (e.g., "rtl-sdr", "hackrf")
	//   - deviceID: Unique identifier of the device (e.g., serial number)
	//   - config: Optional device configuration. Can be string, []byte, or JSON-serializable object
	//
	// Returns:
	//   - sessionID: Unique identifier for the created session
	//   - error: If session creation fails or context is cancelled
	CreateSession(ctx context.Context, deviceType, deviceID string, config any) (sessionID int64, err error)

	// Session retrieves a specific scanning session by its ID.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeouts
	//   - id: Unique session identifier
	//
	// Returns:
	//   - session: Pointer to session data, nil if not found
	//   - error: If retrieval fails or context is cancelled
	Session(ctx context.Context, id int64) (session *spectrum.ScanSession, err error)

	// Sessions returns all scanning sessions stored in the database.
	// Results are ordered by start time in ascending order.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeouts
	//
	// Returns:
	//   - sessions: Slice of pointers to session data
	//   - error: If retrieval fails or context is cancelled
	Sessions(ctx context.Context) (sessions []*spectrum.ScanSession, err error)

	// StoreTelemetry saves drone telemetry data for a specific session.
	// The telemetry data is linked to spectrum measurements for position correlation.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeouts
	//   - sessionID: ID of the session this telemetry belongs to
	//   - t: Telemetry data containing drone sensors readings
	//
	// Returns:
	//   - telemetryID: Unique identifier for the stored telemetry record
	//   - error: If storage fails or context is cancelled
	StoreTelemetry(ctx context.Context, sessionID int64, t *telemetry.Telemetry) (telemetryID int64, err error)

	// StoreSweepResult saves spectrum sweep data, optionally linked to telemetry.
	// All readings in the sweep result are stored in a single atomic transaction.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeouts
	//   - sessionID: ID of the session this sweep belongs to
	//   - telemetryID: Optional ID linking to concurrent telemetry data
	//   - result: Sweep result containing frequency power readings
	//
	// Returns:
	//   - error: If storage fails or context is cancelled
	StoreSweepResult(ctx context.Context, sessionID int64, telemetryID *int64, result *sdr.SweepResult) error

	// Close releases all database connections and resources.
	// After Close is called, the store instance cannot be reused.
	// It is safe to call Close multiple times.
	//
	// Returns:
	//   - error: If closing fails or some resources cannot be released
	Close() error
}

// Store handles database operations
type store struct {
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
func New(dbPath string) (Store, error) {
	return &store{dbPath: dbPath}, nil
}

func initSchema(db *sql.DB) error {
	_, err := db.Exec(schemaSQL)
	return err
}

func (s *store) getWriteDB() (*sql.DB, error) {
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

func (s *store) getReadDB() (*sql.DB, error) {
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

func closeWithError(closer io.Closer, err *error) {
	if cErr := closer.Close(); cErr != nil && *err == nil {
		*err = cErr
	}
}

func (s *store) CreateSession(ctx context.Context, deviceType, deviceID string, config any) (sessionID int64, err error) {
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

	stmt, err := db.PrepareContext(ctx, insertSessionSQL)
	if err != nil {
		err = fmt.Errorf("preparing statement: %w", err)
		return
	}
	defer closeWithError(stmt, &err)

	result, err := stmt.ExecContext(ctx, deviceType, deviceID, configData)
	if err != nil {
		err = fmt.Errorf("inserting session: %w", err)
		return
	}

	sessionID, err = result.LastInsertId()
	if err != nil {
		err = fmt.Errorf("getting session ID: %w", err)
	}
	return
}

func (s *store) Session(ctx context.Context, id int64) (session *spectrum.ScanSession, err error) {
	db, err := s.getReadDB()
	if err != nil {
		err = fmt.Errorf("getting read connection: %w", err)
		return
	}

	stmt, err := db.PrepareContext(ctx, selectSessionSQL)
	if err != nil {
		err = fmt.Errorf("preparing statement: %w", err)
		return
	}
	defer closeWithError(stmt, &err)

	var sess spectrum.ScanSession
	var config sql.NullString
	if err = stmt.QueryRowContext(ctx, id).Scan(&sess.ID, &sess.StartTime, &sess.DeviceType, &sess.DeviceID, &config); err != nil {
		err = fmt.Errorf("scanning session: %w", err)
		return
	}
	if config.Valid {
		sess.Config = &config.String
	}

	return &sess, nil
}

func (s *store) Sessions(ctx context.Context) (sessions []*spectrum.ScanSession, err error) {
	db, err := s.getReadDB()
	if err != nil {
		err = fmt.Errorf("getting read connection: %w", err)
		return
	}

	rows, err := db.QueryContext(ctx, selectSessionsSQL)
	if err != nil {
		err = fmt.Errorf("querying sessions: %w", err)
		return
	}
	defer closeWithError(rows, &err)

	for rows.Next() {
		var sess spectrum.ScanSession
		var config sql.NullString
		if err = rows.Scan(&sess.ID, &sess.StartTime, &sess.DeviceType, &sess.DeviceID, &config); err != nil {
			err = fmt.Errorf("scanning session: %w", err)
			return
		}
		if config.Valid {
			sess.Config = &config.String
		}
		sessions = append(sessions, &sess)
	}
	return
}

func (s *store) StoreTelemetry(ctx context.Context, sessionID int64, t *telemetry.Telemetry) (telemetryID int64, err error) {
	db, err := s.getWriteDB()
	if err != nil {
		err = fmt.Errorf("getting write connection: %w", err)
		return
	}

	stmt, err := db.PrepareContext(ctx, insertTelemetrySQL)
	if err != nil {
		err = fmt.Errorf("preparing statement: %w", err)
		return
	}
	defer closeWithError(stmt, &err)

	data := toTelemetryData(sessionID, t)

	result, err := stmt.ExecContext(
		ctx,
		data.SessionID,
		data.Timestamp,
		data.Latitude,
		data.Longitude,
		data.Altitude,
		data.Roll,
		data.Pitch,
		data.Yaw,
		data.AccelX,
		data.AccelY,
		data.AccelZ,
		data.GroundSpeed,
		data.GroundCourse,
		data.RadioRSSI,
	)
	if err != nil {
		err = fmt.Errorf("inserting telemetry: %w", err)
		return
	}

	telemetryID, err = result.LastInsertId()
	if err != nil {
		err = fmt.Errorf("getting telemetry ID: %w", err)
	}
	return
}

func (s *store) StoreSweepResult(ctx context.Context, sessionID int64, telemetryID *int64, result *sdr.SweepResult) (err error) {
	if len(result.Readings) == 0 {
		return
	}

	db, err := s.getWriteDB()
	if err != nil {
		return fmt.Errorf("getting write connection: %w", err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() {
		if cErr := tx.Rollback(); cErr != nil && err == nil {
			err = cErr
		}
	}()

	stmt, err := tx.PrepareContext(ctx, insertSampleSQL)
	if err != nil {
		return fmt.Errorf("preparing statement: %w", err)
	}
	defer closeWithError(stmt, &err)

	for _, sample := range result.Readings {
		data := toSampleData(sessionID, telemetryID, sample, result)

		_, err = stmt.ExecContext(
			ctx,
			data.SessionID,
			data.Timestamp,
			data.Frequency,
			data.BinWidth,
			data.Power,
			data.NumSamples,
			data.TelemetryID,
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

func (s *store) Close() error {
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
