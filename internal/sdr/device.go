package sdr

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
)

const (
	// ParseErrorsThreshold defines the number of consecutive parse errors allowed
	ParseErrorsThreshold = 5
)

var (
	// ErrTooManyParseErrors is returned when the number of consecutive parse errors exceeds the threshold
	ErrTooManyParseErrors = errors.New("too many consecutive parse errors")

	// ErrBrokenPipe is returned when there's an error reading from stdout or stderr
	ErrBrokenPipe = errors.New("broken pipe")
)

// Handler defines the interface for managing and interacting with SDR devices.
// It provides methods for command execution, output parsing, and device configuration.
// Each implementation should handle device-specific details like command-line arguments
// and output format parsing.
type Handler interface {
	// Cmd returns an exec.Cmd configured to run the device's command-line tool.
	// The command should be properly configured with all necessary arguments and
	// environment variables. The context allows for cancellation of long-running commands.
	//
	// Parameters:
	//   - ctx: Context for command cancellation and timeout control
	//
	// Returns an exec.Cmd ready for execution.
	Cmd(ctx context.Context) *exec.Cmd

	// Parse processes a single line of output from the device's command-line tool.
	// It converts the raw output into structured sweep results containing frequency
	// and power measurements.
	//
	// Parameters:
	//   - line: Raw text line from device output
	//   - deviceID: Unique identifier of the device producing the output
	//   - samples: Channel for sending parsed sweep results
	//
	// Returns error if parsing fails or the output format is invalid.
	Parse(line string, deviceID string) (*SweepResult, error)

	// Device returns the identifier or type of the SDR device being handled
	// (e.g., "rtl-sdr", "hackrf", etc.).
	//
	// Returns a string identifying the device type.
	Device() string

	// Runtime returns the name or path of the command-line tool used to
	// control the device (e.g., "rtl_power", "hackrf_sweep").
	//
	// Returns the command name or full path to the executable.
	Runtime() string

	// Args returns the list of command-line arguments needed to run the
	// device's command-line tool with the desired configuration.
	//
	// Returns a slice of strings containing the command-line arguments.
	Args() []string
}

// DeviceOption represents a functional option for configuring a Device.
type DeviceOption func(*Device)

// WithLogger sets the logger for the device
func WithLogger(logger *slog.Logger) func(d *Device) {
	return func(d *Device) {
		d.logger = logger.With(
			slog.String("device", d.handler.Device()),
			slog.String("deviceID", d.deviceID),
		)
	}
}

// WithParseErrorsThreshold sets the threshold for consecutive parse errors
func WithParseErrorsThreshold(threshold uint8) func(d *Device) {
	return func(d *Device) {
		d.parseErrorsThreshold = threshold
	}
}

func WithBuffer(buffer *SweepsBuffer) func(d *Device) {
	return func(d *Device) {
		d.buffer = buffer
	}
}

// Device struct represents an SDR device that can be started (samples collection) and stopped
type Device struct {
	deviceID string
	handler  Handler
	buffer   *SweepsBuffer

	isSampling atomic.Bool
	cancel     context.CancelFunc
	wg         sync.WaitGroup

	parseErrorsThreshold uint8
	logger               *slog.Logger
}

// NewDevice creates a new Device instance with a discard logger
func NewDevice(deviceID string, h Handler, opts ...DeviceOption) *Device {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil)) // nil logger

	d := &Device{
		deviceID:             deviceID,
		handler:              h,
		logger:               logger,
		parseErrorsThreshold: ParseErrorsThreshold,
	}

	for _, opt := range opts {
		opt(d)
	}
	return d
}

// DeviceID returns the device ID
func (d *Device) DeviceID() string {
	return d.deviceID
}

// Device returns the device name / type
func (d *Device) Device() string {
	return d.handler.Device()
}

// BeginSampling starts the device and collects samples, sending them to the samples channel
func (d *Device) BeginSampling(ctx context.Context, sr chan<- *SweepResult) (<-chan error, error) {
	if d.isSampling.Load() {
		return nil, fmt.Errorf("device is already running")
	}

	d.isSampling.Store(true)

	ctx, d.cancel = context.WithCancel(ctx)
	cmd := d.handler.Cmd(ctx)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		d.isSampling.Store(false) // Reset running state on error
		return nil, fmt.Errorf("error creating stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		d.isSampling.Store(false) // Reset running state on error
		return nil, fmt.Errorf("error creating stderr pipe: %w", err)
	}

	if err = cmd.Start(); err != nil {
		d.isSampling.Store(false) // Reset running state on error
		return nil, fmt.Errorf("error starting command: %w", err)
	}

	d.logger.Info("command started",
		slog.String("runtime", d.handler.Runtime()),
		slog.String("args", strings.Join(d.handler.Args(), " ")))

	samplingStopped := make(chan error)

	d.wg.Add(1)
	go func() {
		done := make(chan error, 3) // expects three results from three goroutines

		go d.handleStdout(stdout, d.deviceID, sr, done)
		go d.handleStderr(stderr, done)
		go d.handleCmdWait(cmd, done)

		var errs []error
		for i := 0; i < cap(done); i++ {
			if err := <-done; err != nil {
				d.cancel() // cancel context on error
				d.logger.Error(err.Error())

				errs = append(errs, err)
			}
		}

		close(done)

		d.logger.Info("samples collection stopped")

		d.isSampling.Store(false)
		d.wg.Done()

		if len(errs) > 0 {
			samplingStopped <- errors.Join(errs...)
		}

		close(samplingStopped)
	}()

	return samplingStopped, nil
}

func (d *Device) Stop() {
	if !d.isSampling.Load() {
		return // already stopped
	}

	d.cancel()
	d.wg.Wait()
	d.isSampling.Store(false)
}

// IsSampling returns true if the device is running
func (d *Device) IsSampling() bool {
	return d.isSampling.Load()
}

// handleStdout reads from stdout, parses and sends samples to the samples channel.
func (d *Device) handleStdout(stdout io.Reader, deviceID string, sr chan<- *SweepResult, done chan<- error) {
	var parseErrors uint8

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)

		if line == "" {
			continue
		}

		sweep, err := d.handler.Parse(line, deviceID)
		if err != nil {
			parseErrors++
			d.logger.Warn(fmt.Sprintf("error parsing samples: %s", err.Error()), slog.String("line", line))

			if parseErrors >= d.parseErrorsThreshold {
				done <- ErrTooManyParseErrors
				return
			}

			continue
		}

		parseErrors = 0 // reset counter

		if d.buffer == nil {
			sr <- sweep
			continue
		}
		if err = d.buffer.Insert(sweep); err != nil {
			d.logger.Warn(fmt.Sprintf("inserting sweep into the buffer: %s", err.Error()), slog.String("line", line))
			continue
		}
		if d.buffer.IsFull() {
			for _, s := range d.buffer.Flush() {
				sr <- s
			}
		}
	}
	if d.buffer != nil && d.buffer.Size() > 0 {
		for _, s := range d.buffer.Drain() {
			sr <- s
		}
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, fs.ErrClosed) {
		done <- fmt.Errorf("%w: reading stdout: %w", ErrBrokenPipe, err)
		return
	}

	done <- nil
}

// handleStderr reads from stderr and logs errors.
func (d *Device) handleStderr(stderr io.Reader, done chan<- error) {
	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)

		if line == "" {
			continue
		}

		d.logger.Warn(fmt.Sprintf("%s >> %s", d.handler.Device(), line)) // simple logging here
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, fs.ErrClosed) {
		done <- fmt.Errorf("%w: error reading stderr: %w", ErrBrokenPipe, err)
		return
	}

	done <- nil
}

// handleCmdWait waits for the command to exit and sends the error to the error channel
func (d *Device) handleCmdWait(cmd *exec.Cmd, done chan<- error) {
	if err := cmd.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		done <- fmt.Errorf("command exited with error: %w", err)
		return
	}

	done <- nil
}
