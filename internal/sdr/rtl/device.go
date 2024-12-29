package rtl

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"gtihub.con/roman-kulish/radio-surveillance/internal/sdr"
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
func New(deviceIndex int, config *Config) (sdr.Handler, error) {
	binPath, err := sdr.FindRuntime(Runtime)
	if err != nil {
		return nil, fmt.Errorf("error finding runtime: %w", err)
	}

	args, err := config.Args(deviceIndex)
	if err != nil {
		return nil, fmt.Errorf("error creating args: %w", err)
	}

	return &handler{binPath, args}, nil
}

// Cmd returns an exec.Cmd for the RTL-SDR handler
func (h handler) Cmd(ctx context.Context) *exec.Cmd {
	return exec.CommandContext(ctx, h.binPath, h.args...)
}

// Parse parses a line of RTL-SDR output and sends samples to the channel
func (h handler) Parse(line string, deviceID string, sr chan<- sdr.SweepResult) error {
	fields := strings.Split(line, ",")
	if len(fields) < 7 {
		return fmt.Errorf("invalid rtl_power output: not enough fields")
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
		return fmt.Errorf("invalid timestamp: %w", err)
	}

	// Parse low frequency, bin information and number of samples
	// Note that the high frequency is not used, because the low frequency and
	// bin size are used to calculate the center frequency of each bin.
	freqLow, err := strconv.ParseFloat(strings.TrimSpace(fields[2]), 64)
	if err != nil {
		return fmt.Errorf("invalis start frequency: %w", err)
	}

	result.BinWidth, err = strconv.ParseFloat(strings.TrimSpace(fields[4]), 64)
	if err != nil {
		return fmt.Errorf("invalid bin width: %w", err)
	}

	result.NumSamples, err = strconv.Atoi(strings.TrimSpace(fields[5]))
	if err != nil {
		return fmt.Errorf("invalid number of samples: %w", err)
	}

	// Parse average power values
	for i, field := range fields[6:] {
		reading := sdr.PowerReading{
			Frequency: freqLow + (float64(i) * result.BinWidth) + (result.BinWidth / 2),
		}

		if power, err := strconv.ParseFloat(strings.TrimSpace(field), 64); err == nil {
			reading.Power = power
			reading.IsValid = true
		}

		result.Samples = append(result.Samples, reading)
	}

	sr <- result
	return nil
}

func (h handler) Device() string {
	return Device
}
