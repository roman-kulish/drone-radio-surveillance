package hackrf

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	MinNumSamples = 8192
	MaxLNAGain    = 40
	MaxVGAGain    = 62
	LNAGainStep   = 8
	VGAGainStep   = 2
)

// ConfigError is a custom error type for configuration errors
type ConfigError struct {
	msg string
}

func (e *ConfigError) Error() string {
	return e.msg
}

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
	FrequencyStart int64 // -f freq_min Frequency range start in MHz
	FrequencyEnd   int64 // -f freq_max Frequency range end in MHz

	// Important but Optional (have reasonable defaults)
	LNAGain    *int  // -l gain_db LNA (IF) gain, 0-40dB, 8dB steps
	VGAGain    *int  // -g gain_db VGA (baseband) gain, 0-62dB, 2dB steps
	BinWidth   int64 // -w bin_width FFT bin width (frequency resolution) in Hz
	NumSamples int64 // -n num_samples Number of samples per frequency, 8192-4294967296

	// Optional - Advanced Configuration
	SerialNumber string // -d serial_number Serial number of desired HackRF
	EnableAmp    bool   // -a amp_enable RX RF amplifier 1=Enable, 0=Disable
	AntennaPower bool   // -p antenna_enable Antenna port power, 1=Enable, 0=Disable

	// Always run scan continuously
	// OneShot      bool   // -1 One shot mode

	NumSweeps int // -N num_sweeps Number of sweeps to perform

	// For the sake of consistency with `rtl_power`,
	// BinaryOutput bool // -B Binary output
	// InverseFFT   bool // -I Binary inverse FFT output

	// Always dump to stdout
	// OutputFile   string // -r filename Output file

	// FFTW wisdom file support (-W and -P options) is not implemented
	// Normalized timestamp option (-n) is not supported to keep behaviour consistent with `rtl_power`

	// Example invocation:
	// hackrf_sweep -f 824:849 -w 100k -l 16 -g 20
}

func (c *Config) Validate() error {
	// Frequency range validation
	if c.FrequencyStart >= c.FrequencyEnd {
		return &ConfigError{"hackrf.Config: frequency end must be greater than frequency start"}
	}

	// LNA gain validation (0-40dB in 8dB steps)
	if c.LNAGain != nil {
		if *c.LNAGain < 0 || *c.LNAGain > MaxLNAGain {
			return &ConfigError{fmt.Sprintf("hackrf.Config: LNA gain must be between 0 and 40 dB: %d given", *c.LNAGain)}
		}
		if *c.LNAGain%LNAGainStep != 0 {
			return &ConfigError{"hackrf.Config: LNA gain must be a multiple of 8 dB"}
		}
	}

	// VGA gain validation (0-62dB in 2dB steps)
	if c.VGAGain != nil {
		if *c.VGAGain < 0 || *c.VGAGain > MaxVGAGain {
			return &ConfigError{fmt.Sprintf("hackrf.Config: VGA gain must be between 0 and 62 dB: %d given", *c.VGAGain)}
		}
		if *c.VGAGain%VGAGainStep != 0 {
			return &ConfigError{"hackrf.Config: VGA gain must be a multiple of 2 dB"}
		}
	}

	// NumSamples validation (if specified)
	if c.NumSamples > 0 && c.NumSamples < MinNumSamples {
		return &ConfigError{fmt.Sprintf("hackrf.Config: number of samples must be at least 8192: %d given", c.NumSamples)}
	}

	// NumSweeps validation (if specified)
	if c.NumSweeps < 0 {
		return &ConfigError{fmt.Sprintf("hackrf.Config: number of sweeps cannot be negative: %d given", c.NumSweeps)}
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

	if c.NumSamples >= 8192 {
		args = append(args, "-n", strconv.FormatInt(c.NumSamples, 10))
	}

	if c.EnableAmp {
		args = append(args, "-a", "1")
	}

	if c.AntennaPower {
		args = append(args, "-p", "1")
	}

	// Always run scan continuously
	// if c.OneShot {
	// 	args = append(args, "-1")
	// }

	if c.NumSweeps > 0 {
		args = append(args, "-N", strconv.Itoa(c.NumSweeps))
	}

	// For the sake of consistency with `rtl_power`,
	// if c.BinaryOutput {
	// 	args = append(args, "-B")
	// }
	//
	// if c.InverseFFT {
	// 	args = append(args, "-I")
	// }

	// Always dump to stdout
	// if c.OutputFile != "" {
	// 	args = append(args, "-r", c.OutputFile)
	// }

	return args, nil
}

func (c *Config) String() string {
	args, err := c.Args()
	if err != nil {
		return fmt.Sprintf("hackrf.Config: invalid config: %v", err)
	}
	return "hackrf_sweep " + strings.Join(args, " ")
}
