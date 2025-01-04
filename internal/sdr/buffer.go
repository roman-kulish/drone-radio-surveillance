package sdr

import (
	"container/list"
	"fmt"
	"sync"
	"time"
)

// SweepsBuffer implements a thread-safe buffer for storing SDR frequency sweep results
// in correct frequency order while handling sweep rollovers. It maintains sweeps
// in order based on their frequency ranges and timestamps, automatically handling
// cases where sweep chunks arrive out of order or span across frequency rollover points.
type SweepsBuffer struct {
	baseFreq float64 // Minimum frequency in Hz for the sweep range
	maxFreq  float64 // Maximum frequency in Hz for the sweep range

	capacity   int // Maximum number of sweeps to store
	flushCount int // Number of sweeps to remove when buffer reaches capacity

	mu   sync.Mutex
	list *list.List
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
func NewFrequencyBuffer(startFreq, endFreq float64, capacity, flushCount int) (*SweepsBuffer, error) {
	if capacity <= 0 || flushCount <= 0 || flushCount > capacity {
		return nil, fmt.Errorf("invalid buffer parameters: bufferCap=%d, toFlush=%d", capacity, flushCount)
	}
	if startFreq >= endFreq {
		return nil, fmt.Errorf("invalid frequency range: start=%f, end=%f", startFreq, endFreq)
	}
	return &SweepsBuffer{
		baseFreq:   startFreq,
		maxFreq:    endFreq,
		capacity:   capacity,
		flushCount: flushCount,
		list:       list.New(),
	}, nil
}

// Insert adds a new frequency sweep to the buffer in the correct order.
// It maintains sweep order based on frequency ranges and ensures temporal consistency
// within each sweep sequence. Returns an error if the sweep is nil.
func (fb *SweepsBuffer) Insert(sweep *SweepResult) error {
	if sweep == nil {
		return fmt.Errorf("cannot insert nil sweep")
	}

	fb.mu.Lock()
	defer fb.mu.Unlock()

	// First element case
	if fb.list.Len() == 0 {
		fb.list.PushFront(sweep)
		return nil
	}

	// Special case: if chunk belongs before head
	if fb.compareSweepOrder(sweep, fb.list.Front().Value.(*SweepResult)) == -1 {
		fb.list.PushFront(sweep)
		return nil
	}

	// Find insertion point
	for e := fb.list.Front(); e != nil; e = e.Next() {
		// If we're at the end or the next chunk should come after our new chunk
		if e.Next() == nil || fb.compareSweepOrder(e.Next().Value.(*SweepResult), sweep) == 1 {
			// Ensure temporal consistency
			if sweep.Timestamp.Before(e.Value.(*SweepResult).Timestamp) {
				sweep.Timestamp = e.Value.(*SweepResult).Timestamp.Add(time.Microsecond)
			}

			fb.list.InsertAfter(sweep, e)
			return nil
		}
	}

	return nil
}

// IsFull returns true if the buffer has reached its capacity.
func (fb *SweepsBuffer) IsFull() bool {
	fb.mu.Lock()
	defer fb.mu.Unlock()

	return fb.list.Len() >= fb.capacity
}

// Flush removes and returns the oldest sweeps from the buffer.
// Returns nil if the buffer is empty. The number of sweeps returned
// is determined by the flushCount parameter and buffer state.
func (fb *SweepsBuffer) Flush() []*SweepResult {
	fb.mu.Lock()
	defer fb.mu.Unlock()

	if fb.list.Len() == 0 {
		return nil
	}

	count := fb.flushCount
	if fb.list.Len() > fb.capacity {
		count += fb.list.Len() - fb.capacity
	}
	count = min(count, fb.list.Len()) // Ensure we don't exceed available items

	results := make([]*SweepResult, 0, count) // Preallocate with capacity

	for i := 0; i < count; i++ {
		front := fb.list.Front()
		if front == nil {
			break
		}
		results = append(results, front.Value.(*SweepResult))
		fb.list.Remove(front)
	}

	return results
}

// Drain removes and returns all sweeps from the buffer.
// Returns nil if the buffer is empty.
func (fb *SweepsBuffer) Drain() []*SweepResult {
	fb.mu.Lock()
	defer fb.mu.Unlock()

	if fb.list.Len() == 0 {
		return nil
	}

	results := make([]*SweepResult, 0, fb.list.Len())
	for e := fb.list.Front(); e != nil; e = e.Next() {
		results = append(results, e.Value.(*SweepResult))
	}

	fb.list.Init() // Clear the list
	return results
}

// Size returns the current number of sweeps in the buffer.
func (fb *SweepsBuffer) Size() int {
	fb.mu.Lock()
	defer fb.mu.Unlock()
	return fb.list.Len()
}

// Clear removes all sweeps from the buffer.
func (fb *SweepsBuffer) Clear() {
	fb.mu.Lock()
	defer fb.mu.Unlock()
	fb.list.Init()
}

// getSweepOrder calculates the relative position of a sweep in the frequency range.
// Returns the sweep's position index based on its center frequency.
func (fb *SweepsBuffer) getSweepOrder(s *SweepResult) int {
	if s == nil {
		return 0
	}
	return int((s.CenterFrequency() - fb.baseFreq) / s.BinWidth)
}

// isNewSweep determines if the given sweep represents the start of a new frequency sweep sequence.
// It uses a rollover threshold calculated as half of the total frequency range divided by the bin width.
// A sweep is considered new when its order (position in frequency range) falls below this threshold,
// indicating that the frequency has rolled over from high to low, starting a new sweep sequence.
//
// Parameters:
//   - s: sweep result to check
//
// Returns true if the sweep's frequency indicates it belongs to a new sweep sequence,
// false if it's part of the current sweep sequence.
func (fb *SweepsBuffer) isNewSweep(s *SweepResult) bool {
	rolloverThreshold := int((fb.maxFreq - fb.baseFreq) / s.BinWidth / 2)
	return fb.getSweepOrder(s) < rolloverThreshold
}

// compareSweepOrder determines the relative ordering of two sweeps.
// Returns:
//
//	1 if 'a' belongs after 'b' (next in sequence or new sweep)
//	-1 if 'a' belongs before 'b' (part of previous sweep)
//	0 if either sweep is nil
func (fb *SweepsBuffer) compareSweepOrder(a, b *SweepResult) int {
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
