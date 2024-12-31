package app

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"slices"
	"sync"

	"github.con/roman-kulish/radio-surveillance/internal/sdr"
	"github.con/roman-kulish/radio-surveillance/internal/sdr/hackrf"
	"github.con/roman-kulish/radio-surveillance/internal/sdr/rtl"
	"github.con/roman-kulish/radio-surveillance/internal/storage"
	"github.con/roman-kulish/radio-surveillance/internal/telemetry"
)

const maxBatchSize = 100

// WithMaxBatchSize sets the maximum batch size of collected samples to store
// within a single database transaction.
func WithMaxBatchSize(size int) func(*Orchestrator) {
	return func(o *Orchestrator) {
		o.maxBatchSize = size
	}
}

// WithTelemetry sets the telemetry provider to use for enriching sweep results
func WithTelemetry(provider telemetry.Provider) func(*Orchestrator) {
	return func(o *Orchestrator) {
		o.telemetry = provider
	}
}

// Orchestrator represents an orchestrator that manages the sweep process
// across multiple devices, optionally enriches sweep results with telemetry
// data, from a drone, and stores the results in a database.
type Orchestrator struct {
	devices  []*sdr.Device
	configs  map[string]any
	sessions map[string]int64

	logger    *slog.Logger
	store     *storage.Store
	telemetry telemetry.Provider

	maxBatchSize int

	wg     sync.WaitGroup
	cancel context.CancelFunc
}

// NewOrchestrator creates a new Orchestrator
func NewOrchestrator(store *storage.Store, logger *slog.Logger, options ...func(*Orchestrator)) *Orchestrator {
	d := Orchestrator{
		configs:      make(map[string]any),
		sessions:     make(map[string]int64),
		logger:       logger,
		store:        store,
		maxBatchSize: maxBatchSize,
	}

	for _, option := range options {
		option(&d)
	}

	return &d
}

// CreateDevice creates a new device and registers it with the Orchestrator
func (o *Orchestrator) CreateDevice(config *DeviceConfig) error {
	if !config.Enabled {
		return nil
	}

	var handler sdr.Handler
	var err error
	switch config.Type {
	case DeviceRTLSDR:
		if handler, err = rtl.New(config.Config.(*rtl.Config)); err != nil {
			return fmt.Errorf("creating RTL-SDR Device: %w", err)
		}

	case DeviceHackRF:
		if handler, err = hackrf.New(config.Config.(*hackrf.Config)); err != nil {
			return fmt.Errorf("creating HackRF Device: %w", err)
		}

	default:
		return fmt.Errorf("creating Device: unknown type '%s'", config.Type)
	}

	device := sdr.NewDevice(config.Name, handler, sdr.WithLogger(o.logger))
	if _, ok := o.configs[device.DeviceID()]; ok {
		return fmt.Errorf("device %s already exists", config.Name)
	}

	o.devices = append(o.devices, device)
	o.configs[config.Name] = config.Config

	return nil
}

// Run begins synchronized data collection across all devices
func (o *Orchestrator) Run(ctx context.Context) error {
	if len(o.devices) == 0 {
		return fmt.Errorf("no devices to sample")
	}
	for _, device := range o.devices {
		sessionID, err := o.store.CreateSession(device.Device(), device.DeviceID(), o.configs[device.DeviceID()])
		if err != nil {
			return fmt.Errorf("creating session for device %s: %w", device.DeviceID(), err)
		}

		o.sessions[device.DeviceID()] = sessionID
	}

	ctx, o.cancel = context.WithCancel(ctx)
	startGate := make(chan struct{})
	samples := make(chan sdr.SweepResult, len(o.devices))

	go o.handleSweepResults(samples)

	for _, device := range o.devices {
		o.wg.Add(1)
		go o.beginSampling(ctx, device, samples, startGate)
	}

	close(startGate) // Start the sampling goroutines

	o.wg.Wait()
	o.cancel()

	close(samples) // Close the samples channel and signal the goroutines to stop
	clear(o.sessions)
	return nil
}

func (o *Orchestrator) beginSampling(ctx context.Context, dev *sdr.Device, samples chan<- sdr.SweepResult, startGate chan struct{}) {
	defer o.wg.Done()

	<-startGate

	// TODO: implement a watchdog to detect if a device is not running and restart it

	done, err := dev.BeginSampling(ctx, samples)
	if err != nil {
		o.logger.Error(err.Error())
		o.cancel() // signal to other goroutines about fatal
		return
	}

	<-done // Wait for the device sampling goroutine to finish
}

func (o *Orchestrator) handleSweepResults(samples chan sdr.SweepResult) {
	for sample := range samples {
		if err := o.storeSweepResult(sample); err != nil {
			o.logger.Error(err.Error())
		}
	}
}

func (o *Orchestrator) storeSweepResult(r sdr.SweepResult) error {
	sessionID := o.sessions[r.DeviceID]

	var telemetryID sql.NullInt64
	if o.telemetry != nil {
		data := telemetryToModel(o.telemetry.Get())
		data.SessionID = sessionID

		if id, err := o.store.InsertTelemetry(data); err != nil {
			o.logger.Error(err.Error())
		} else {
			telemetryID = sql.NullInt64{
				Int64: id,
				Valid: true,
			}
		}
	}

	data := make([]storage.SampleData, len(r.Readings))
	for i, reading := range r.Readings {
		data[i] = storage.SampleData{
			SessionID: sessionID,
			Timestamp: r.Timestamp.UTC(),
			Frequency: reading.Frequency,
			BinWidth:  r.BinWidth,
			Power: sql.NullFloat64{
				Float64: reading.Power,
				Valid:   reading.IsValid,
			},
			NumSamples:  r.NumSamples,
			TelemetryID: telemetryID,
		}
	}

	for chunk := range slices.Chunk(data, o.maxBatchSize) {
		if err := o.store.BatchInsertSamples(chunk); err != nil {
			return fmt.Errorf("storing samples: %w", err)
		}
	}

	return nil
}

func telemetryToModel(t *telemetry.Telemetry) storage.TelemetryData {
	var td storage.TelemetryData

	td.Timestamp = t.Timestamp.UTC()

	td.Latitude = sql.NullFloat64{
		Float64: *t.Latitude,
		Valid:   t.Latitude != nil,
	}

	td.Longitude = sql.NullFloat64{
		Float64: *t.Longitude,
		Valid:   t.Longitude != nil,
	}

	td.Altitude = sql.NullFloat64{
		Float64: *t.Altitude,
		Valid:   t.Altitude != nil,
	}

	td.Roll = sql.NullFloat64{
		Float64: *t.Roll,
		Valid:   t.Roll != nil,
	}

	td.Pitch = sql.NullFloat64{
		Float64: *t.Pitch,
		Valid:   t.Pitch != nil,
	}

	td.Yaw = sql.NullFloat64{
		Float64: *t.Yaw,
		Valid:   t.Yaw != nil,
	}

	td.AccelX = sql.NullFloat64{
		Float64: *t.AccelX,
		Valid:   t.AccelX != nil,
	}
	td.AccelY = sql.NullFloat64{
		Float64: *t.AccelY,
		Valid:   t.AccelY != nil,
	}

	td.AccelZ = sql.NullFloat64{
		Float64: *t.AccelZ,
		Valid:   t.AccelZ != nil,
	}

	td.GroundSpeed = sql.NullInt16{
		Int16: int16(*t.GroundSpeed),
		Valid: t.GroundSpeed != nil,
	}

	td.GroundCourse = sql.NullInt16{
		Int16: int16(*t.GroundCourse),
		Valid: t.GroundCourse != nil,
	}

	td.RadioRSSI = sql.NullInt16{
		Int16: int16(*t.RadioRSSI),
		Valid: t.RadioRSSI != nil,
	}

	return td
}
