//go:build linux

package hackrf

import (
	"fmt"

	"gtihub.con/roman-kulish/radio-surveillance/internal/sdr/driver"
)

func findBinary() (string, error) {
	binPath, err := exec.LookPath(runtime)
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return "", driver.NewRuntimeError(fmt.Sprintf("hackrf: `%s` not found in PATH: %w", runtime, err.Error()))
		}
		return "", driver.NewRuntimeError(fmt.Sprintf("hackrf: failed to locate binary: %w", &err.Error()))
	}

	return binPath, nil
}
