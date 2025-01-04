package rtl

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/roman-kulish/radio-surveillance/internal/sdr"
)

const (
	Runtime = "rtl_power"
	Device  = "RTL-SDR"
)

// handler struct represents an RTL-SDR handler
type handler struct {
	binPath string
	args    []string
}

// New creates a new RTL-SDR handler
func New(config *Config) (sdr.Handler, error) {
	binPath, err := sdr.FindRuntime(Runtime)
	if err != nil {
		return nil, fmt.Errorf("error finding runtime: %w", err)
	}

	args, err := config.Args()
	if err != nil {
		return nil, fmt.Errorf("error creating args: %w", err)
	}

	return &handler{binPath, args}, nil
}

// Cmd returns an exec.Cmd configured to run the device's command-line tool
func (h handler) Cmd(ctx context.Context) *exec.Cmd {
	return exec.CommandContext(ctx, h.binPath, h.args...)
}

// Parse processes a single line of output from the device's command-line tool
func (h handler) Parse(line string, deviceID string) (*sdr.SweepResult, error) {
	fields := strings.Split(line, ",")
	if len(fields) < 7 {
		return nil, fmt.Errorf("invalid rtl_power output: not enough fields")
	}

	var err error

	result := sdr.SweepResult{
		Device:   Device,
		DeviceID: deviceID,
	}

	// Parse timestamp
	dateTime := strings.TrimSpace(fields[0]) + " " + strings.TrimSpace(fields[1])
	result.Timestamp, err = time.Parse("2006-01-02 15:04:05", dateTime)
	if err != nil {
		return nil, fmt.Errorf("invalid timestamp: %w", err)
	}

	// Parse low frequency, bin information and number of samples
	result.StartFrequency, err = strconv.ParseFloat(strings.TrimSpace(fields[2]), 64)
	if err != nil {
		return nil, fmt.Errorf("invalid start frequency: %w", err)
	}

	result.EndFrequency, err = strconv.ParseFloat(strings.TrimSpace(fields[3]), 64)
	if err != nil {
		return nil, fmt.Errorf("invalid end frequency: %w", err)
	}

	result.BinWidth, err = strconv.ParseFloat(strings.TrimSpace(fields[4]), 64)
	if err != nil {
		return nil, fmt.Errorf("invalid bin width: %w", err)
	}

	result.NumSamples, err = strconv.Atoi(strings.TrimSpace(fields[5]))
	if err != nil {
		return nil, fmt.Errorf("invalid number of samples: %w", err)
	}

	// Parse average power values
	for i, field := range fields[6:] {
		reading := sdr.PowerReading{
			Frequency: result.StartFrequency + (float64(i) * result.BinWidth) + (result.BinWidth / 2),
		}

		if power, err := strconv.ParseFloat(strings.TrimSpace(field), 64); err == nil {
			reading.Power = power
			reading.IsValid = true
		}

		result.Readings = append(result.Readings, reading)
	}

	return &result, nil
}

// Device returns the identifier or type of the SDR device being handled
func (h handler) Device() string {
	return Device
}

// Runtime returns the name or path of the command-line tool used to
// control the device (e.g., "rtl_power", "hackrf_sweep")
func (h handler) Runtime() string {
	return h.binPath
}

// Args returns the list of command-line arguments needed to run the
// device's command-line tool with the desired configuration
func (h handler) Args() []string {
	return h.args
}
