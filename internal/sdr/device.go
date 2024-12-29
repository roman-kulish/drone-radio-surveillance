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

// Handler interface defines the methods required for handling a device
type Handler interface {
	Cmd(ctx context.Context) *exec.Cmd
	Parse(line string, deviceID string, samples chan<- Sample) error
	Device() string
}

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

// Device struct represents an SDR device that can be started (samples collection) and stopped
type Device struct {
	deviceID string
	handler  Handler

	isSampling atomic.Bool
	cancel     context.CancelFunc
	wg         sync.WaitGroup

	parseErrorsThreshold uint8
	logger               *slog.Logger
}

// NewDevice creates a new Device instance with a discard logger
func NewDevice(deviceID string, h Handler, options ...func(d *Device)) *Device {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil)) // nil logger

	d := Device{
		deviceID:             deviceID,
		handler:              h,
		logger:               logger,
		parseErrorsThreshold: ParseErrorsThreshold,
	}

	for _, option := range options {
		option(&d)
	}

	return &d
}

// BeginSampling starts the device and collects samples, sending them to the samples channel
func (d *Device) BeginSampling(ctx context.Context, samples chan<- Sample) (<-chan error, error) {
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

	samplingStopped := make(chan error)

	d.wg.Add(1)
	go func() {
		defer close(samplingStopped)

		d.logger.Info("starting samples collection...")

		done := make(chan error, 3) // expects three results from three goroutines

		go d.handleStdout(stdout, d.deviceID, samples, done)
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
func (d *Device) handleStdout(stdout io.Reader, deviceID string, samples chan<- Sample, done chan<- error) {
	var parseErrors uint8

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)

		if line == "" {
			continue
		}

		if err := d.handler.Parse(line, deviceID, samples); err != nil {
			parseErrors++
			d.logger.Warn(fmt.Sprintf("error parsing samples: %s", err.Error()), slog.String("line", line))

			if parseErrors >= d.parseErrorsThreshold {
				done <- ErrTooManyParseErrors
				return
			}

			continue
		}

		parseErrors = 0 // reset counter
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, fs.ErrClosed) {
		done <- fmt.Errorf("%w: error reading stdout: %w", ErrBrokenPipe, err)
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
