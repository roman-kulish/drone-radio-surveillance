package hackrf

import (
	"context"
	"fmt"
	"os/exec"

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
	// TODO implement me
	fmt.Println(line)
	return nil
}

func (h handler) Device() string {
	return Device
}
