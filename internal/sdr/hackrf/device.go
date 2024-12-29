package hackrf

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
	Runtime = "hackrf_sweep"
	Device  = "HackRF"
)

type handler struct {
	binPath string
	args    []string
}

func New(serialNumber string, config *Config) (sdr.Handler, error) {
	binPath, err := sdr.FindRuntime(Runtime)
	if err != nil {
		return nil, fmt.Errorf("error finding runtime: %w", err)
	}

	args, err := config.Args(serialNumber)
	if err != nil {
		return nil, fmt.Errorf("error creating args: %w", err)
	}

	return &handler{binPath, args}, nil
}

func (h handler) Cmd(ctx context.Context) *exec.Cmd {
	return exec.CommandContext(ctx, h.binPath, h.args...)
}

func (h handler) Parse(line string, deviceID string, samples chan<- sdr.Sample) error {
	fields := strings.Split(line, ",")
	if len(fields) < 7 {
		return fmt.Errorf("invalid rtl_power output: not enough fields")
	}

	// Parse timestamp
	dateTime := strings.TrimSpace(fields[0]) + " " + strings.TrimSpace(fields[1])
	timestamp, err := time.Parse("2006-01-02 15:04:05.000000", dateTime)
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

	binWidth, err := strconv.ParseFloat(strings.TrimSpace(fields[4]), 64)
	if err != nil {
		return fmt.Errorf("invalid bin width: %w", err)
	}

	numSamples, err := strconv.Atoi(strings.TrimSpace(fields[5]))
	if err != nil {
		return fmt.Errorf("invalid number of samples: %w", err)
	}

	// Parse average power values
	for i, field := range fields[6:] {
		power, err := strconv.ParseFloat(strings.TrimSpace(field), 64)
		if err != nil {
			continue // Skip invalid power readings
		}

		// Calculate center frequency for this bin
		centerFreq := freqLow + (float64(i) * binWidth) + (binWidth / 2)

		sample := sdr.Sample{
			Timestamp:  timestamp,
			Frequency:  centerFreq,
			Power:      power,
			BinWidth:   binWidth,
			NumSamples: numSamples,
			Device:     Device,
			DeviceID:   deviceID,
		}

		samples <- sample
	}

	return nil
}

func (h handler) Device() string {
	return Device
}
