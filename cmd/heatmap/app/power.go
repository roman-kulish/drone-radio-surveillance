package app

import (
	"math"
	"sync"
)

// PowerBounds represents the calculated power boundaries
type PowerBounds struct {
	Min       float64
	Max       float64
	Mean      float64
	Reference float64
}

func defaultPowerBounds() PowerBounds {
	return PowerBounds{
		Min:       -120,
		Max:       -90,
		Mean:      -105,
		Reference: -105,
	}
}

// PowerHistogram maintains a histogram of power values with 1dBm bins
type PowerHistogram struct {
	// Using int64 for counts to handle very large datasets
	bins       map[int]int64 // Map of bin index to count
	totalCount int64

	// Cache for min/max to avoid map iteration
	minBin int
	maxBin int

	mu sync.RWMutex
}

// NewPowerHistogram creates a new histogram
func NewPowerHistogram() *PowerHistogram {
	return &PowerHistogram{
		bins:   make(map[int]int64),
		minBin: math.MaxInt32,
		maxBin: math.MinInt32,
	}
}

// getBinIndex converts power value to bin index
func getBinIndex(power float64) int {
	return int(math.Floor(power)) // 1dBm bins
}

// Update adds new power reading to the histogram
func (h *PowerHistogram) Update(power *float64) {
	if power == nil {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	// Update bins
	bin := getBinIndex(*power)
	h.bins[bin]++
	h.totalCount++

	// Update min/max bins
	if bin < h.minBin {
		h.minBin = bin
	}
	if bin > h.maxBin {
		h.maxBin = bin
	}
}

// Clear resets the histogram
func (h *PowerHistogram) Clear() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.bins = make(map[int]int64)
	h.totalCount = 0
	h.minBin = math.MaxInt32
	h.maxBin = math.MinInt32
}

// GetPercentileBounds returns power bounds based on percentiles
func (h *PowerHistogram) GetPercentileBounds() PowerBounds {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.totalCount == 0 {
		return defaultPowerBounds()
	}

	// Calculate target counts for 5th and 95th percentiles
	target5th := h.totalCount * 5 / 100

	// Find the bins corresponding to these percentiles
	var count int64
	var min5th, max95th int

	// Find 5th percentile
	for bin := h.minBin; bin <= h.maxBin; bin++ {
		count += h.bins[bin]
		if count >= target5th {
			min5th = bin
			break
		}
	}

	// Find 95th percentile
	count = 0
	for bin := h.maxBin; bin >= h.minBin; bin-- {
		count += h.bins[bin]
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
	alpha   float64
	current PowerBounds
	mu      sync.RWMutex
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

	s.mu.Lock()
	defer s.mu.Unlock()

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
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.current
}

// Clear resets the histogram and bounds
func (s *SmoothBounds) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.hist.Clear()
	s.current = defaultPowerBounds()
}
