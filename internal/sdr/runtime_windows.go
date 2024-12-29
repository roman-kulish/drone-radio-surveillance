//go:build windows && amd64

package sdr

import (
	"fmt"
	"os"
	"path/filepath"
)

func FindRuntime(runtime string) (string, error) {
	lookup := []string{}

	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to get executable path: %w", err)
	}

	lookup = append(lookup, filepath.Dir(exePath))

	exePath, err = os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current working directory: %w", err)
	}

	lookup = append(lookup, exePath)

	for _, exeDir := range lookup {
		matches, err := filepath.Glob(filepath.Join(exeDir, "bin", "*", "windows", "x64", fmt.Sprintf("%s.exe", runtime)))
		if err != nil || len(matches) == 0 {
			continue // continue to next directory
		}

		binPath := matches[0]
		if _, err = os.Stat(binPath); err != nil {
			continue // continue to next directory
		}

		return binPath, nil
	}

	return "", fmt.Errorf("failed to find binary '%s'", runtime)
}
