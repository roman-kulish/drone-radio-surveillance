package sdr

import "time"

// PowerReading represents a single frequency power reading,
// allowing for explicit invalid/missing data representation
type PowerReading struct {
	Frequency float64 // Center frequency in Hz
	Power     float64 // Power level (dBm for rtl_sdr, dB for hackrf)
	IsValid   bool    // Whether the sample is valid
}

// SweepResult represents a single sample from the SDR
type SweepResult struct {
	Timestamp      time.Time      // Timestamp information
	StartFrequency float64        // StartFrequency specifies the starting frequency in Hz for the sweep
	EndFrequency   float64        // EndFrequency specifies the ending frequency in Hz for the sweep
	BinWidth       float64        // Hz step/bin width
	NumSamples     int            // Number of samples used for this measurement
	Readings       []PowerReading // Samples contains a collection of power readings for a sweep result
	Device         string         // Device type (e.g., "rtl-sdr", "hackrf")
	DeviceID       string         // Serial number or index (human-readable)
}
