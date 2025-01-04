package sdr

import (
	"fmt"
	"sync"
	"time"
)

// Node represents an internal linked list node for the frequency sweep buffer.
type node struct {
	sweep *SweepResult
	next  *node
}

// FrequencyBuffer implements a thread-safe buffer for storing SDR frequency sweep results
// in correct frequency order while handling sweep rollovers. It maintains sweeps
// in order based on their frequency ranges and timestamps, automatically handling
// cases where sweep chunks arrive out of order or span across frequency rollover points.
type FrequencyBuffer struct {
	baseFreq float64 // Minimum frequency in Hz for the sweep range
	maxFreq  float64 // Maximum frequency in Hz for the sweep range

	capacity   int // Maximum number of sweeps to store
	flushCount int // Number of sweeps to remove when buffer reaches capacity

	mu   sync.Mutex
	head *node
	size int
}

// NewFrequencyBuffer creates a new frequency sweep buffer for the specified frequency range.
// The buffer will store up to capacity sweeps and remove flushCount sweeps when full.
//
// Parameters:
//   - startFreq: minimum frequency in Hz
//   - endFreq: maximum frequency in Hz
//   - capacity: maximum number of sweeps to store
//   - flushCount: number of sweeps to remove when buffer is full
//
// Returns an error if parameters are invalid.
func NewFrequencyBuffer(startFreq, endFreq float64, capacity, flushCount int) (*FrequencyBuffer, error) {
	if capacity <= 0 || flushCount <= 0 || flushCount > capacity {
		return nil, fmt.Errorf("invalid buffer parameters: bufferCap=%d, toFlush=%d", capacity, flushCount)
	}
	if startFreq >= endFreq {
		return nil, fmt.Errorf("invalid frequency range: start=%f, end=%f", startFreq, endFreq)
	}
	return &FrequencyBuffer{
		baseFreq:   startFreq,
		maxFreq:    endFreq,
		capacity:   capacity,
		flushCount: flushCount,
	}, nil
}

// Insert adds a new frequency sweep to the buffer in the correct order.
// It maintains sweep order based on frequency ranges and ensures temporal consistency
// within each sweep sequence. Returns an error if the sweep is nil.
func (fb *FrequencyBuffer) Insert(sweep *SweepResult) error {
	if sweep == nil {
		return fmt.Errorf("cannot insert nil sweep")
	}

	fb.mu.Lock()
	defer fb.mu.Unlock()

	if fb.head == nil {
		fb.head = &node{sweep: sweep}
		fb.size++
		return nil
	}

	// Special case: if chunk belongs before head
	if fb.compareSweepOrder(sweep, fb.head.sweep) == -1 {
		n := &node{sweep: sweep, next: fb.head}
		fb.head = n
		fb.size++
		return nil
	}

	// Find insertion point
	current := fb.head
	for current != nil {
		// If we're at the end or the next chunk should come after our new chunk
		if current.next == nil || fb.compareSweepOrder(current.next.sweep, sweep) == 1 {
			// Ensure temporal consistency
			if sweep.Timestamp.Before(current.sweep.Timestamp) {
				sweep.Timestamp = current.sweep.Timestamp.Add(time.Microsecond)
			}

			n := &node{sweep: sweep, next: current.next}
			current.next = n
			fb.size++
			return nil
		}
		current = current.next
	}

	return nil
}

// IsFull returns true if the buffer has reached its capacity.
func (fb *FrequencyBuffer) IsFull() bool {
	fb.mu.Lock()
	defer fb.mu.Unlock()

	return fb.size >= fb.capacity
}

// Flush removes and returns the oldest sweeps from the buffer.
// Returns nil if the buffer is empty. The number of sweeps returned
// is determined by the flushCount parameter and buffer state.
func (fb *FrequencyBuffer) Flush() []*SweepResult {
	fb.mu.Lock()
	defer fb.mu.Unlock()

	if fb.head == nil || fb.size == 0 {
		return nil
	}

	count := fb.flushCount
	if fb.size > fb.capacity {
		count += fb.size - fb.capacity
	}
	count = min(count, fb.size) // Ensure we don't exceed available items

	results := make([]*SweepResult, 0, count) // Preallocate with capacity
	current := fb.head
	for i := 0; i < count && current != nil; i++ {
		results = append(results, current.sweep)
		current = current.next
	}

	fb.head = current
	fb.size -= len(results)
	return results
}

// DrainAll removes and returns all sweeps from the buffer.
// Returns nil if the buffer is empty.
func (fb *FrequencyBuffer) DrainAll() []*SweepResult {
	fb.mu.Lock()
	defer fb.mu.Unlock()

	if fb.head == nil || fb.size == 0 {
		return nil
	}

	results := make([]*SweepResult, 0, fb.size) // Preallocate with capacity
	current := fb.head
	for current != nil {
		results = append(results, current.sweep)
		current = current.next
	}

	fb.head = nil
	fb.size = 0
	return results
}

// Size returns the current number of sweeps in the buffer.
func (fb *FrequencyBuffer) Size() int {
	fb.mu.Lock()
	defer fb.mu.Unlock()
	return fb.size
}

// Clear removes all sweeps from the buffer.
func (fb *FrequencyBuffer) Clear() {
	fb.mu.Lock()
	defer fb.mu.Unlock()
	fb.head = nil
	fb.size = 0
}

// getSweepOrder calculates the relative position of a sweep in the frequency range.
// Returns the sweep's position index based on its center frequency.
func (fb *FrequencyBuffer) getSweepOrder(s *SweepResult) int {
	if s == nil {
		return 0
	}
	return int((s.CenterFrequency() - fb.baseFreq) / s.BinWidth)
}

// compareSweepOrder determines the relative ordering of two sweeps.
// Returns:
//
//	1 if 'a' belongs after 'b' (next in sequence or new sweep)
//	-1 if 'a' belongs before 'b' (part of previous sweep)
//	0 if either sweep is nil
func (fb *FrequencyBuffer) compareSweepOrder(a, b *SweepResult) int {
	if a == nil || b == nil {
		return 0
	}

	ac := fb.getSweepOrder(a)
	bc := fb.getSweepOrder(b)

	rolloverThreshold := int((fb.maxFreq - fb.baseFreq) / a.BinWidth / 2)

	ar := ac < rolloverThreshold
	br := bc < rolloverThreshold

	switch {
	case ar && br:
		if ac > bc {
			return 1
		}
		return -1

	case ar:
		return 1

	case br:
		return -1

	default:
		if ac > bc {
			return 1
		}
		return -1
	}
}
