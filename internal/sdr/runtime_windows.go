//go:build windows && amd64

package sdr

import (
	"fmt"
	"os"
	"path/filepath"
)

func FindRuntime(runtime string) (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current working directory: %w", err)
	}

	matches, err := filepath.Glob(filepath.Join(wd, "bin", "*", "windows", "x64", fmt.Sprintf("%s.exe", runtime)))
	if err != nil || len(matches) == 0 {
		return "", fmt.Errorf("failed to find binary '%s'", runtime)
	}

	binPath := matches[0]
	if _, err = os.Stat(binPath); err != nil {
		return "", fmt.Errorf("failed to stat binary '%s': %w", binPath, err)
	}

	return binPath, nil
}
