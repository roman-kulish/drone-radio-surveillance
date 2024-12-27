//go:build linux

package driver

import (
	"errors"
	"os/exec"
)

func FindRuntime(runtime string) (string, error) {
	binPath, err := exec.LookPath(runtime)
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return "", err
		}
		return "", err
	}

	return binPath, nil
}
