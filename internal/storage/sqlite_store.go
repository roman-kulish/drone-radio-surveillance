package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/roman-kulish/radio-surveillance/internal/sdr"
	"github.com/roman-kulish/radio-surveillance/internal/spectrum"
	"github.com/roman-kulish/radio-surveillance/internal/telemetry"
)

// SqliteStore handles database operations
type SqliteStore struct {
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

// NewSqliteStore creates a new database connection and initializes the schema
// using the Sqlite database
func NewSqliteStore(dbPath string) *SqliteStore {
	return &SqliteStore{dbPath: dbPath}
}

func runSQLCommand(db *sql.DB, sql string) error {
	_, err := db.Exec(sql)
	return err
}

func (s *SqliteStore) getWriteDB() (*sql.DB, error) {
	s.writeDBOnce.Do(func() {
		db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?%s", s.dbPath, "_journal_mode=WAL&_synchronous=NORMAL"))
		if err != nil {
			s.writeDBErr = fmt.Errorf("opening write connection: %w", err)
			return
		}

		if err = runSQLCommand(db, initSchemaSQL); err != nil {
			_ = db.Close()
			s.writeDBErr = fmt.Errorf("initializing schema: %w", err)
			return
		}

		s.writeDB = db
	})

	return s.writeDB, s.writeDBErr
}

func (s *SqliteStore) getReadDB() (*sql.DB, error) {
	s.readDBOnce.Do(func() {
		db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?%s", s.dbPath, "mode=ro"))
		if err != nil {
			s.readDBErr = fmt.Errorf("opening read connection: %w", err)
			return
		}
		s.readDB = db
	})

	return s.readDB, s.readDBErr
}

func (s *SqliteStore) CreateSession(ctx context.Context, deviceType, deviceID string, config any) (sessionID int64, err error) {
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

func (s *SqliteStore) Session(ctx context.Context, id int64) (session *spectrum.ScanSession, err error) {
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

func (s *SqliteStore) Sessions(ctx context.Context) (sessions []*spectrum.ScanSession, err error) {
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

// ReadSpectrum creates a new SpectrumReader that provides access to basic spectral measurements
// from a scanning session. The reader implements efficient iteration over large datasets through
// pagination and supports various filtering and sorting options.
//
// Parameters:
//   - ctx: Context for cancellation and timeouts
//   - sessionID: Unique identifier of the scanning session to read from
//   - opts: Optional configuration parameters for the reader (WithTimeRange, WithFrequencyRange,
//     WithBatchSize, WithSortOrder)
//
// The returned SpectrumReader must be closed after use to release database resources.
// It is safe to call from multiple goroutines, but each reader instance should only be
// used from a single goroutine.
//
// Returns error if reader creation fails or session doesn't exist.
func (s *SqliteStore) ReadSpectrum(ctx context.Context, sessionID int64, opts ...ReaderOption[spectrum.SpectralPoint]) (*SqliteSpectrumReader[spectrum.SpectralPoint], error) {
	db, err := s.getReadDB()
	if err != nil {
		return nil, fmt.Errorf("getting read connection: %w", err)
	}
	return newSqliteSpectrumReader[spectrum.SpectralPoint](db, sessionID, false, opts...)
}

// ReadSpectrumWithTelemetry creates a new SpectrumReader that provides access to spectral
// measurements enriched with drone telemetry data. Each point includes position, orientation,
// and radio link quality information captured during the measurement.
//
// Parameters:
//   - ctx: Context for cancellation and timeouts
//   - sessionID: Unique identifier of the scanning session to read from
//   - opts: Optional configuration parameters for the reader (supports all ReadSpectrum options
//     plus WithAltitudeRange, WithPositionBounds, WithMinimumSignalQuality)
//
// The returned SpectrumReader must be closed after use to release database resources.
// Telemetry data is joined with spectral data using nearest-time matching.
// It is safe to call from multiple goroutines, but each reader instance should only be
// used from a single goroutine.
//
// Returns error if reader creation fails, session doesn't exist, or telemetry data is unavailable.
func (s *SqliteStore) ReadSpectrumWithTelemetry(ctx context.Context, sessionID int64, opts ...ReaderOption[spectrum.SpectralPointWithTelemetry]) (*SqliteSpectrumReader[spectrum.SpectralPointWithTelemetry], error) {
	db, err := s.getReadDB()
	if err != nil {
		return nil, fmt.Errorf("getting read connection: %w", err)
	}
	return newSqliteSpectrumReader[spectrum.SpectralPointWithTelemetry](db, sessionID, true, opts...)
}

func (s *SqliteStore) StoreTelemetry(ctx context.Context, sessionID int64, t *telemetry.Telemetry) (telemetryID int64, err error) {
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

const insertSampleSQL = `
    INSERT INTO samples (
        session_id,
        timestamp,
        frequency,
        bin_width,
        power,
        num_samples,
        telemetry_id
    )
    VALUES `

func (s *SqliteStore) StoreSweepResult(ctx context.Context, sessionID int64, telemetryID *int64, result *sdr.SweepResult) (err error) {
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
	defer rollbackWithError(tx, &err)

	// Prepare values array
	values := make([]interface{}, 0, len(result.Readings)*7)

	// Build batch insert query
	valuesPlaceholder := "(?, ?, ?, ?, ?, ?, ?)"

	var sb strings.Builder

	sb.WriteString(insertSampleSQL)

	for i, sample := range result.Readings {
		data := toSampleData(sessionID, telemetryID, sample, result)
		values = append(values,
			data.SessionID,
			data.Timestamp,
			data.Frequency,
			data.BinWidth,
			data.Power,
			data.NumSamples,
			data.TelemetryID,
		)

		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(valuesPlaceholder)
	}

	// Single batch insert
	if _, err = tx.ExecContext(ctx, sb.String(), values...); err != nil {
		return fmt.Errorf("batch inserting samples: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	return nil
}

func (s *SqliteStore) Close() error {
	s.closeOnce.Do(func() {
		var writeErr, readErr error

		if s.writeDB != nil {
			_ = runSQLCommand(s.writeDB, initIndexesSQL)

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
