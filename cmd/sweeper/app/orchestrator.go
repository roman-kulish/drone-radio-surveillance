package app

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/roman-kulish/radio-surveillance/internal/sdr"
	"github.com/roman-kulish/radio-surveillance/internal/sdr/hackrf"
	"github.com/roman-kulish/radio-surveillance/internal/sdr/rtl"
	"github.com/roman-kulish/radio-surveillance/internal/storage"
	"github.com/roman-kulish/radio-surveillance/internal/telemetry"
)

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
	store     storage.Store
	telemetry telemetry.Provider

	wg     sync.WaitGroup
	cancel context.CancelFunc
}

// NewOrchestrator creates a new Orchestrator
func NewOrchestrator(store storage.Store, logger *slog.Logger, options ...func(*Orchestrator)) *Orchestrator {
	d := Orchestrator{
		configs:  make(map[string]any),
		sessions: make(map[string]int64),
		logger:   logger,
		store:    store,
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

	ctx, o.cancel = context.WithCancel(ctx)

	for _, device := range o.devices {
		sessionID, err := o.store.CreateSession(ctx, device.Device(), device.DeviceID(), o.configs[device.DeviceID()])
		if err != nil {
			return fmt.Errorf("creating session for device %s: %w", device.DeviceID(), err)
		}

		o.sessions[device.DeviceID()] = sessionID
	}

	startGate := make(chan struct{})
	samples := make(chan *sdr.SweepResult, len(o.devices))

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

func (o *Orchestrator) beginSampling(ctx context.Context, dev *sdr.Device, samples chan<- *sdr.SweepResult, startGate chan struct{}) {
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

func (o *Orchestrator) handleSweepResults(samples chan *sdr.SweepResult) {
	for sample := range samples {
		// This function MUST drain the channel and persist all the data.
		if err := o.storeSweepResult(context.Background(), sample); err != nil {
			o.logger.Error(err.Error())
		}
	}
}

func (o *Orchestrator) storeSweepResult(ctx context.Context, r *sdr.SweepResult) error {
	sessionID := o.sessions[r.DeviceID]

	var telemetryID *int64
	if o.telemetry != nil {
		if tm := o.telemetry.Get(); tm != nil {
			id, err := o.store.StoreTelemetry(ctx, sessionID, o.telemetry.Get())
			if err != nil {
				o.logger.Error(err.Error())
			} else {
				telemetryID = &id
			}
		}
	}

	return o.store.StoreSweepResult(ctx, sessionID, telemetryID, r)
}
