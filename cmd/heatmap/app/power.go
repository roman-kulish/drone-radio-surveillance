package app

import "math"

const (
	defaultMinPower = -120.0 // dBm
	defaultMaxPower = -20.0  // dBm

	// For 20 samples:
	// - 5% percentile  = 1 sample
	// - 95% percentile = 19th sample
	minimumSampleCount = 20
)

// PowerBounds represents the calculated power boundaries
type PowerBounds struct {
	Min       float64 // 5th percentile power level in dBm
	Max       float64 // 95th percentile power level in dBm
	Mean      float64 // Mean power level in dBm
	Reference float64 // Reference level for visualization in dBm
}

func defaultPowerBounds() PowerBounds {
	return PowerBounds{
		Min:       defaultMinPower,
		Max:       defaultMaxPower,
		Mean:      (defaultMinPower + defaultMaxPower) / 2,
		Reference: (defaultMinPower + defaultMaxPower) / 2,
	}
}

// PowerHistogram maintains a histogram of power values with 1dBm bins
type PowerHistogram struct {
	bins       map[int]uint32 // Map of bin index to count
	totalCount uint64         // Total number of samples
	minBin     int            // Cache for min bin
	maxBin     int            // Cache for max bin
}

// NewPowerHistogram creates a new histogram
func NewPowerHistogram() *PowerHistogram {
	return &PowerHistogram{
		bins:   make(map[int]uint32),
		minBin: math.MaxInt32,
		maxBin: math.MinInt32,
	}
}

// getBinIndex converts power value to bin index
func getBinIndex(power float64) int {
	return int(math.Floor(power)) // 1dBm bins
}

// scaleDown scales all bin counts down by factor of 2
func (h *PowerHistogram) scaleDown() {
	h.minBin = math.MaxInt32
	h.maxBin = math.MinInt32

	// Scale down all bins by factor of 2
	for bin := range h.bins {
		h.bins[bin] /= 2
		// Remove bin if it becomes 0
		if h.bins[bin] == 0 {
			delete(h.bins, bin)
		}

		if bin < h.minBin {
			h.minBin = bin
		}
		if bin > h.maxBin {
			h.maxBin = bin
		}
	}
	h.totalCount /= 2
}

// Update adds new power reading to the histogram
func (h *PowerHistogram) Update(power *float64) {
	if power == nil {
		return
	}

	bin := getBinIndex(*power)

	// Check both conditions for scaling
	if h.bins[bin] == math.MaxUint32 || h.totalCount == math.MaxUint64 {
		h.scaleDown()
	}

	h.bins[bin]++
	h.totalCount++

	if bin < h.minBin {
		h.minBin = bin
	}
	if bin > h.maxBin {
		h.maxBin = bin
	}
}

// Clear resets the histogram
func (h *PowerHistogram) Clear() {
	h.bins = make(map[int]uint32)
	h.totalCount = 0
	h.minBin = math.MaxInt32
	h.maxBin = math.MinInt32
}

// GetPercentileBounds returns power bounds based on percentiles
func (h *PowerHistogram) GetPercentileBounds() PowerBounds {
	if h.totalCount < minimumSampleCount { // Require minimum samples
		return defaultPowerBounds()
	}

	// Calculate target counts for 5th and 95th percentiles
	target5th := h.totalCount * 5 / 100

	// Find the bins corresponding to these percentiles
	var count uint64
	var min5th, max95th int

	// Find 5th percentile
	for bin := h.minBin; bin <= h.maxBin; bin++ {
		count += uint64(h.bins[bin])
		if count >= target5th {
			min5th = bin
			break
		}
	}

	// Find 95th percentile
	count = 0
	for bin := h.maxBin; bin >= h.minBin; bin-- {
		count += uint64(h.bins[bin])
		if count >= target5th {
			max95th = bin
			break
		}
	}

	// Calculate mean (weighted average of bin centers)
	var sumProduct float64
	for bin := h.minBin; bin <= h.maxBin; bin++ {
		sumProduct += float64(bin) * float64(h.bins[bin])
	}
	mean := sumProduct / float64(h.totalCount)

	// Ensure minimum range of 30dB
	if max95th-min5th < 30 {
		center := (max95th + min5th) / 2
		min5th = center - 15
		max95th = center + 15
	}

	// Add small margin
	margin := (max95th - min5th) * 1 / 10 // 10% margin
	minPower := float64(min5th - margin)
	maxPower := float64(max95th + margin)

	return PowerBounds{
		Min:       minPower,
		Max:       maxPower,
		Mean:      mean,
		Reference: mean,
	}
}

// SmoothBounds represents a smoothed version of the histogram bounds
type SmoothBounds struct {
	hist    *PowerHistogram
	alpha   float64     // Smoothing factor (0-1)
	current PowerBounds // Current smoothed bounds
}

// NewSmoothBounds creates a new bounds smoother
func NewSmoothBounds(alpha float64) *SmoothBounds {
	return &SmoothBounds{
		hist:    NewPowerHistogram(),
		alpha:   alpha,
		current: defaultPowerBounds(),
	}
}

// Update adds new power reading and returns smoothed bounds
func (s *SmoothBounds) Update(power *float64) PowerBounds {
	if power == nil {
		return s.current
	}

	// Update histogram
	s.hist.Update(power)

	// Get new bounds
	newBounds := s.hist.GetPercentileBounds()

	// Apply exponential smoothing
	s.current.Min = s.current.Min*(1-s.alpha) + newBounds.Min*s.alpha
	s.current.Max = s.current.Max*(1-s.alpha) + newBounds.Max*s.alpha
	s.current.Mean = newBounds.Mean // Use new mean directly
	s.current.Reference = newBounds.Reference

	return s.current
}

// Current returns the current smoothed power bounds
func (s *SmoothBounds) Current() PowerBounds {
	return s.current
}

// Clear resets the histogram and bounds
func (s *SmoothBounds) Clear() {
	s.hist.Clear()
	s.current = defaultPowerBounds()
}
