package hackrf

import (
	"errors"
	"fmt"
	"strconv"
)

const (
	MaxLNAGain  = 40
	MaxVGAGain  = 62
	LNAGainStep = 8
	VGAGainStep = 2
)

// Usage examples from man page:
// https://manpages.debian.org/bookworm/hackrf/hackrf_sweep.1.en.html

/*
	hackrfConfig := hackrf.Config{
        FrequencyStart: 824_000_000,  // 824 MHz
        FrequencyEnd:   849_000_000,  // 849 MHz
        BinWidth:       100_000,      // 100 kHz
        Gain:           16,
        VGAGain:        20,
    }
    args := BuildHackRFSweepCommand(hackrfConfig)
    // Executes: hackrf_sweep -f 824:849 -w 100k -l 16 -g 20
*/

// Config is a struct for configuring the `hackrf_sweep` tool
type Config struct {
	// Required
	FrequencyStart int64 `yaml:"frequencyStart" json:"frequencyStart"` // -f freq_min Frequency range start in MHz
	FrequencyEnd   int64 `yaml:"frequencyEnd" json:"frequencyEnd"`     // -f freq_max Frequency range end in MHz

	// Important but Optional (have reasonable defaults)
	LNAGain  *int  `yaml:"lnaGain" json:"lnaGain,omitempty"`   // -l gain_db LNA (IF) gain, 0-40dB, 8dB steps
	VGAGain  *int  `yaml:"vgaGain" json:"vgaGain,omitempty"`   // -g gain_db VGA (baseband) gain, 0-62dB, 2dB steps
	BinWidth int64 `yaml:"binWidth" json:"binWidth,omitempty"` // -w bin_width FFT bin width (frequency resolution) in Hz

	// Optional - Advanced Configuration
	SerialNumber string `yaml:"serialNumber" json:"serialNumber,omitempty"` // -d serial_number Serial number of desired HackRF
	EnableAmp    bool   `yaml:"enableAmp" json:"enableAmp,omitempty"`       // -a amp_enable RX RF amplifier 1=Enable, 0=Disable
	AntennaPower bool   `yaml:"antennaPower" json:"antennaPower,omitempty"` // -p antenna_enable Antenna port power, 1=Enable, 0=Disable
}

func (c *Config) Validate() error {
	// Frequency range validation
	if c.FrequencyStart >= c.FrequencyEnd {
		return errors.New("hackrf.Config: frequency end must be greater than frequency start")
	}

	// LNA gain validation (0-40dB in 8dB steps)
	if c.LNAGain != nil {
		if *c.LNAGain < 0 || *c.LNAGain > MaxLNAGain {
			return fmt.Errorf("hackrf.Config: LNA gain must be between 0 and 40 dB: %d given", *c.LNAGain)
		}
		if *c.LNAGain%LNAGainStep != 0 {
			return errors.New("hackrf.Config: LNA gain must be a multiple of 8 dB")
		}
	}

	// VGA gain validation (0-62dB in 2dB steps)
	if c.VGAGain != nil {
		if *c.VGAGain < 0 || *c.VGAGain > MaxVGAGain {
			return fmt.Errorf("hackrf.Config: VGA gain must be between 0 and 62 dB: %d given", *c.VGAGain)
		}
		if *c.VGAGain%VGAGainStep != 0 {
			return errors.New("hackrf.Config: VGA gain must be a multiple of 2 dB")
		}
	}

	return nil
}

// Args builds the command line arguments for `hackrf_sweep`
// See `man hackrf_sweep` for more information:
// https://manpages.debian.org/bookworm/hackrf/hackrf_sweep.1.en.html
func (c *Config) Args() ([]string, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}

	args := []string{
		"-f", fmt.Sprintf("%d:%d",
			c.FrequencyStart/1e6,
			c.FrequencyEnd/1e6),
	}

	if c.SerialNumber != "" {
		args = append(args, "-d", c.SerialNumber)
	}

	if c.BinWidth > 0 {
		args = append(args, "-w", strconv.FormatInt(c.BinWidth, 10))
	}

	if c.LNAGain != nil {
		args = append(args, "-l", strconv.Itoa(*c.LNAGain))
	}

	if c.VGAGain != nil {
		args = append(args, "-g", strconv.Itoa(*c.VGAGain))
	}

	if c.EnableAmp {
		args = append(args, "-a", "1")
	}

	if c.AntennaPower {
		args = append(args, "-p", "1")
	}

	return args, nil
}
