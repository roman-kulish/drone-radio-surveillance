//go:build windows && amd64

package hackrf

import (
	"fmt"
	"os"
	"path/filepath"

	"gtihub.con/roman-kulish/radio-surveillance/internal/sdr/driver"
)

func findBinary() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", err
	}

	exeDir := filepath.Dir(exePath)

	// Construct the expected binary path
	binPath := filepath.Join(exeDir, "bin", runtime, "windows", "x64", fmt.Sprintf("%s.exe", runtime))

	// Check if the file exists
	if _, err := os.Stat(binPath); err != nil {
		if os.IsNotExist(err) {
			return "", driver.NewRuntimeError(fmt.Sprintf("hackrf: binary not found at path: %s", binPath))
		}
		return "", driver.NewRuntimeError(fmt.Sprintf("hackrf: failed to stat binary path: %s", err.Error()))
	}
	
	return binPath, nil
}
