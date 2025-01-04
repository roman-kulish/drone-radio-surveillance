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

// CenterFrequency returns the center frequency of the sweep bin.
// The center frequency is calculated as the start frequency plus half of the bin width.
// For example, if the sweep bin starts at 1000 MHz with a width of 200 kHz,
// the center frequency would be 1000.1 MHz.
func (s *SweepResult) CenterFrequency() float64 {
	return s.StartFrequency + (s.BinWidth / 2)
}
