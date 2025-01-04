package storage

import (
	"context"

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
