package rtl

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	BinWidthMin = 1
	BinWidthMax = 2_800_000

	// WindowFunctionRectangle is the default window function
	WindowFunctionRectangle      WindowFunction = "rectangle"
	WindowFunctionHamming        WindowFunction = "hamming"
	WindowFunctionBlackman       WindowFunction = "blackman"
	WindowFunctionBlackmanHarris WindowFunction = "blackman-harris"
	WindowFunctionHannPoisson    WindowFunction = "hann-poisson"
	WindowFunctionBartlett       WindowFunction = "bartlett"
	WindowFunctionYoussef        WindowFunction = "youssef"
	WindowFunctionKaiser         WindowFunction = "kaiser"

	// SmoothingAvg is the default smoothing method
	SmoothingAvg SmoothingMethod = "avg"
	SmoothingIIR SmoothingMethod = "iir"
)

var (
	validWindowFunctions = map[WindowFunction]struct{}{
		WindowFunctionRectangle:      {},
		WindowFunctionHamming:        {},
		WindowFunctionBlackman:       {},
		WindowFunctionBlackmanHarris: {},
		WindowFunctionHannPoisson:    {},
		WindowFunctionYoussef:        {},
		WindowFunctionKaiser:         {},
		WindowFunctionBartlett:       {},
	}

	validSmoothingMethods = map[SmoothingMethod]struct{}{
		SmoothingAvg: {},
		SmoothingIIR: {},
	}
)

type WindowFunction string

func (w WindowFunction) String() string {
	return string(w)
}

type SmoothingMethod string

func (s SmoothingMethod) String() string {
	return string(s)
}

type TimeDuration time.Duration

func NewTimeDuration(d time.Duration) TimeDuration {
	return TimeDuration(d)
}

func (d *TimeDuration) UnmarshalYAML(value *yaml.Node) error {
	duration, err := time.ParseDuration(value.Value)
	if err != nil {
		return fmt.Errorf("rtl.TimeDuration: failed to parse: %s", err)
	}

	*d = TimeDuration(duration)
	return nil
}

func (d *TimeDuration) MarshalYAML() (interface{}, error) {
	return d.String(), nil
}

func (d *TimeDuration) UnmarshalJSON(bytes []byte) error {
	var v string
	if err := json.Unmarshal(bytes, &v); err != nil {
		return err
	}

	duration, err := time.ParseDuration(v)
	if err != nil {
		return fmt.Errorf("rtl.TimeDuration: failed to parse: %s", err)
	}

	*d = TimeDuration(duration)
	return nil
}

func (d *TimeDuration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(*d).String())
}

func (d *TimeDuration) Validate() error {
	duration := time.Duration(*d)

	if duration < 0 {
		return fmt.Errorf("rtl.TimeDuration: must not be negative: %s", duration)
	}
	if duration > 0 && duration < time.Second {
		return fmt.Errorf("rtl.TimeDuration: must be at least 1 second: %s given", duration)
	}

	return nil
}

func (d *TimeDuration) String() string {
	duration := time.Duration(*d)
	if duration%time.Hour == 0 {
		return fmt.Sprintf("%dh", int(duration/time.Hour))
	} else if duration%time.Minute == 0 {
		return fmt.Sprintf("%dm", int(duration/time.Minute))
	} else {
		return fmt.Sprintf("%ds", int(duration/time.Second))
	}
}

// Usage examples from man page:
// https://manpages.debian.org/bookworm/rtl-sdr/rtl_power.1.en.html

/*
Example 1: FM Band Scan
    rtlConfig := rtl.Config{
        FrequencyStart: 88_000_000,   // 88 MHz
        FrequencyEnd:   108_000_000,  // 108 MHz
        BinWidth:        125_000,      // 125 kHz
    }
    // Executes: rtl_power -f 88M:108M:125k -
    // Creates 160 bins across the FM band, individual stations should be visible

Example 2: Wide Range Survey
    rtlConfig := rtl.Config{
        FrequencyStart: 100_000_000,  // 100 MHz
        FrequencyEnd:   1_000_000_000,// 1 GHz
        BinWidth:        1_000_000,    // 1 MHz
        Interval:       "5m",         // 5 minutes
        SingleShot:     true,
    }
    // Executes: rtl_power -f 100M:1G:1M -i 5m -1 -
    // A five-minute low res scan of nearly everything

Example 3: Timed Collection
    rtlConfig := rtl.Config{
        FrequencyStart: 824_000_000,  // 824 MHz
        FrequencyEnd:   849_000_000,  // 849 MHz
        BinWidth:        12_500,       // 12.5 kHz
        Interval:       "15m",        // 15 minutes
        SingleShot:     true,
    }
    // Executes: rtl_power -f ... -i 15m -1 -
    // Integrate for 15 minutes and exit afterward
*/

// Config is the `rtl_power` tool configuration
type Config struct {
	// Required
	FrequencyStart int64 `yaml:"frequencyStart" json:"frequencyStart"` // -f lower Frequency range start (Hz)
	FrequencyEnd   int64 `yaml:"frequencyEnd" json:"frequencyEnd"`     // -f upper Frequency range end (Hz)
	BinWidth       int64 `yaml:"binWidth" json:"binWidth"`             // -f bin_size Bin size in Hz (valid range 1Hz - 2.8MHz)

	// Common Optional Parameters
	Interval TimeDuration `yaml:"interval" json:"interval"` // -i integration_interval (default: 10 seconds)
	// Time units: 's' seconds, 'm' minutes, 'h' hours
	// Default unit is seconds
	// Examples: "30s", "15m", "2h"

	DeviceIndex int `yaml:"deviceIndex" json:"deviceIndex"` // -d device_index (default: 0)

	Gain     int `yaml:"gain" json:"gain"`         // -g tuner_gain (default: automatic)
	PPMError int `yaml:"ppmError" json:"ppmError"` // -p ppm_error (default: 0)

	// Time Control
	ExitTimer TimeDuration `yaml:"exitTimer" json:"exitTimer"` // -e exit_timer (default: off/0)
	// Time units: 's' seconds, 'm' minutes, 'h' hours
	// Default unit is seconds
	// Examples: "30s", "15m", "2h"

	// Processing Options
	Smoothing  SmoothingMethod `yaml:"smoothing" json:"smoothing"`   // -s [avg|iir] Smoothing (default: avg)
	FFTThreads int             `yaml:"fftThreads" json:"fftThreads"` // -t threads Number of FFT threads

	// Advanced/Experimental Options
	WindowFunction WindowFunction `yaml:"windowFunction" json:"windowFunction"` // -w window (default: rectangle)
	Crop           float32        `yaml:"crop" json:"crop"`                     // -c crop_percent (default: 0%, recommended: 20%-50%)
	FIRSize        *int           `yaml:"firSize" json:"firSize"`               // -F fir_size (default: disabled, can be 0 or 9)

	// Hardware Options
	PeakHold       bool `yaml:"peakHold" json:"peakHold"`             // -P enables peak hold (default: off)
	DirectSampling bool `yaml:"directSampling" json:"directSampling"` // -D enable direct sampling (default: off)
	OffsetTuning   bool `yaml:"offsetTuning" json:"offsetTuning"`     // -O enable offset tuning (default: off)
	BiasTee        bool `yaml:"biasTee" json:"biasTee"`               // -T enable bias-tee (default: off)
}

func (c *Config) Validate() error {
	// Validate required fields
	if c.FrequencyStart <= 0 {
		return fmt.Errorf("rtl.Config: frequency start must be positive: %d", c.FrequencyStart)
	}
	if c.FrequencyEnd <= 0 {
		return fmt.Errorf("rtl.Config: frequency end must be positive: %d", c.FrequencyEnd)
	}
	if c.FrequencyEnd <= c.FrequencyStart {
		return fmt.Errorf("rtl.Config: frequency end must be greater than start: %d <= %d", c.FrequencyEnd, c.FrequencyStart)
	}

	// Validate bin width
	if c.BinWidth < BinWidthMin || c.BinWidth > BinWidthMax {
		return fmt.Errorf("rtl.Config: invalid bin width: %d, must be between %d and %d Hz", c.BinWidth, BinWidthMin, BinWidthMax)
	}

	// Validate time specifications
	if c.Interval > 0 {
		if err := c.Interval.Validate(); err != nil {
			return fmt.Errorf("rtl.Config: invalid interval: %w", err)
		}
	}
	if c.ExitTimer > 0 {
		if err := c.ExitTimer.Validate(); err != nil {
			return fmt.Errorf("rtl.Config: invalid exit timer: %w", err)
		}
	}

	// Validate window function
	if c.WindowFunction != "" {
		if _, ok := validWindowFunctions[c.WindowFunction]; !ok {
			return fmt.Errorf("rtl.Config: invalid window function: %s", c.WindowFunction)
		}
	}

	// Validate smoothing method
	if c.Smoothing != "" {
		if _, ok := validSmoothingMethods[c.Smoothing]; !ok {
			return fmt.Errorf("rtl.Config: invalid smoothing method: %s", c.Smoothing)
		}
	}

	// Validate crop percent
	if c.Crop < 0 || c.Crop > 1 {
		return fmt.Errorf("rtl.Config: crop percent must be between 0 and 1: %0.2f given", c.Crop)
	}

	// Validate FIR size
	if c.FIRSize != nil && *c.FIRSize != 0 && *c.FIRSize != 9 {
		return fmt.Errorf("rtl.Config: FIR size must be 0 or 9: %d given", *c.FIRSize)
	}

	return nil
}

// Args returns the command line arguments for `rtl_power`
// See `man rtl_power` for more information:
// https://manpages.debian.org/bookworm/rtl-sdr/rtl_power.1.en.html
func (c *Config) Args() ([]string, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}

	args := []string{
		"-f", fmt.Sprintf("%d:%d:%d",
			c.FrequencyStart,
			c.FrequencyEnd,
			c.BinWidth),
	}

	// Common parameters
	if c.Interval > 0 {
		args = append(args, "-i", c.Interval.String())
	}

	args = append(args, "-d", strconv.Itoa(c.DeviceIndex)) // 0 is the default device index

	if c.Gain > 0 {
		args = append(args, "-g", strconv.Itoa(c.Gain))
	}

	if c.PPMError != 0 {
		args = append(args, "-p", strconv.Itoa(c.PPMError))
	}

	if c.ExitTimer > 0 {
		args = append(args, "-e", c.ExitTimer.String())
	}

	// Processing options
	if c.Smoothing != "" {
		args = append(args, "-s", c.Smoothing.String())
	}

	if c.FFTThreads > 0 {
		args = append(args, "-t", strconv.Itoa(c.FFTThreads))
	}

	// Window and filter options
	if c.WindowFunction != "" {
		args = append(args, "-w", c.WindowFunction.String())
	}

	if c.Crop > 0 {
		args = append(args, "-c", strconv.FormatFloat(float64(c.Crop), 'f', 2, 32))
	}

	if c.FIRSize != nil {
		args = append(args, "-F", strconv.Itoa(*c.FIRSize))
	}

	// Hardware options
	if c.PeakHold {
		args = append(args, "-P")
	}

	if c.DirectSampling {
		args = append(args, "-D")
	}

	if c.OffsetTuning {
		args = append(args, "-O")
	}

	if c.BiasTee {
		args = append(args, "-T")
	}

	args = append(args, "-") // Always dump to stdout

	return args, nil
}

func (c *Config) String() string {
	args, err := c.Args()
	if err != nil {
		return fmt.Sprintf("rtl.Config: failed to build args: %s", err)
	}
	return fmt.Sprintf("%s %s", Runtime, strings.Join(args, " "))
}
