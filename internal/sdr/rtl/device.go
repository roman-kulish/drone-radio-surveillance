package rtl

import (
	"context"
	"fmt"
	"os/exec"

	"gtihub.con/roman-kulish/radio-surveillance/internal/sdr"
)

const (
	Runtime = "rtl_power"
	Device  = "RTL-SDR"
)

type handler struct {
	binPath string
	args    []string
}

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

func (h handler) Cmd(ctx context.Context) *exec.Cmd {
	return exec.CommandContext(ctx, h.binPath, h.args...)
}

func (h handler) Parse(line string, samples chan<- sdr.Sample) error {
	// TODO implement me
	fmt.Println(line)
	return nil
}

func (h handler) Device() string {
	return Device
}
