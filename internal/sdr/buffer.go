package sdr

import (
	"container/list"
	"fmt"
	"math"
	"sync"
	"time"
)

// minSpectrumChunksThreshold is the minimum number of chunks in a complete spectrum required
// to reliably detect frequency rollover. For spectrum with fewer chunks, simple frequency
// order comparison is used. The value is determined empirically based on typical spectrum
// characteristics and rollover detection reliability.
const minSpectrumChunksThreshold = 5

// SweepsBuffer implements a thread-safe buffer for storing SDR frequency sweep results
// in correct frequency order while handling sweep rollovers. It maintains sweeps
// in order based on their frequency ranges and timestamps, automatically handling
// cases where sweep chunks arrive out of order or span across frequency rollover points.
type SweepsBuffer struct {
	baseFreq float64 // Minimum frequency in Hz for the sweep range
	maxFreq  float64 // Maximum frequency in Hz for the sweep range
	binWidth float64 // Bin width observed from the sweep results

	capacity   int // Maximum number of sweeps to store
	flushCount int // Number of sweeps to remove when buffer reaches capacity

	mu   sync.Mutex
	list *list.List
}

// NewFrequencyBuffer creates a new frequency sweep buffer for the specified frequency range.
// The buffer will store up to capacity sweeps and remove flushCount sweeps when full.
//
// Parameters:
//   - capacity: maximum number of sweeps to store
//   - flushCount: number of sweeps to remove when buffer is full
//
// Returns an error if parameters are invalid.
func NewFrequencyBuffer(capacity, flushCount int) (*SweepsBuffer, error) {
	if capacity <= 0 || flushCount <= 0 || flushCount > capacity {
		return nil, fmt.Errorf("invalid buffer parameters: bufferCap=%d, toFlush=%d", capacity, flushCount)
	}
	return &SweepsBuffer{
		baseFreq:   math.MaxFloat64,
		maxFreq:    0,
		binWidth:   0,
		capacity:   capacity,
		flushCount: flushCount,
		list:       list.New(),
	}, nil
}

// Insert adds a new frequency sweep to the buffer in the correct order.
// It maintains sweep order based on frequency ranges and ensures temporal consistency
// within each sweep sequence. Returns an error if the sweep is nil.
func (sb *SweepsBuffer) Insert(sweep *SweepResult) error {
	if sweep == nil {
		return fmt.Errorf("cannot insert nil sweep")
	}

	sb.mu.Lock()
	defer sb.mu.Unlock()

	sb.updateFrequencyRange(sweep)

	// First element case
	if sb.list.Len() == 0 {
		sb.list.PushFront(sweep)
		return nil
	}

	// Special case: if chunk belongs before head
	if sb.compareSweepOrder(sweep, sb.list.Front().Value.(*SweepResult)) == -1 {
		sb.list.PushFront(sweep)
		return nil
	}

	// Find insertion point
	for e := sb.list.Front(); e != nil; e = e.Next() {
		// If we're at the end or the next chunk should come after our new chunk
		if e.Next() == nil || sb.compareSweepOrder(e.Next().Value.(*SweepResult), sweep) == 1 {
			// Ensure temporal consistency
			if sweep.Timestamp.Before(e.Value.(*SweepResult).Timestamp) {
				sweep.Timestamp = e.Value.(*SweepResult).Timestamp.Add(time.Microsecond)
			}

			sb.list.InsertAfter(sweep, e)
			return nil
		}
	}

	return nil
}

// IsFull returns true if the buffer has reached its capacity.
func (sb *SweepsBuffer) IsFull() bool {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	return sb.list.Len() >= sb.capacity
}

// Flush removes and returns the oldest sweeps from the buffer.
// Returns nil if the buffer is empty. The number of sweeps returned
// is determined by the flushCount parameter and buffer state.
func (sb *SweepsBuffer) Flush() []*SweepResult {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	if sb.list.Len() == 0 {
		return nil
	}

	count := sb.flushCount
	if sb.list.Len() > sb.capacity {
		count += sb.list.Len() - sb.capacity
	}
	count = min(count, sb.list.Len()) // Ensure we don't exceed available items

	results := make([]*SweepResult, 0, count) // Preallocate with capacity

	for i := 0; i < count; i++ {
		front := sb.list.Front()
		if front == nil {
			break
		}
		results = append(results, front.Value.(*SweepResult))
		sb.list.Remove(front)
	}

	return results
}

// Drain removes and returns all sweeps from the buffer.
// Returns nil if the buffer is empty.
func (sb *SweepsBuffer) Drain() []*SweepResult {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	if sb.list.Len() == 0 {
		return nil
	}

	results := make([]*SweepResult, 0, sb.list.Len())
	for e := sb.list.Front(); e != nil; e = e.Next() {
		results = append(results, e.Value.(*SweepResult))
	}

	sb.list.Init() // Clear the list
	return results
}

// Size returns the current number of sweeps in the buffer.
func (sb *SweepsBuffer) Size() int {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	return sb.list.Len()
}

// Clear removes all sweeps from the buffer.
func (sb *SweepsBuffer) Clear() {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	sb.list.Init()
}

// getSweepOrder calculates the relative position of a sweep in the frequency range.
// Returns the sweep's position index based on its center frequency.
func (sb *SweepsBuffer) getSweepOrder(s *SweepResult) int {
	if s == nil {
		return 0
	}
	return int((s.CenterFrequency() - sb.baseFreq) / sb.binWidth)
}

// compareSweepOrder determines the relative order of two sweeps, handling both regular
// frequency progression and frequency rollover cases. For spectrum with fewer chunks
// than minSpectrumChunksThreshold, it uses simple frequency comparison. For larger
// spectrum, it enables rollover detection to properly handle sweep boundaries.
//
// The function uses a rollover threshold at half of the frequency range to detect
// when a new sweep starts. For small spectrum where rollover detection might be
// unreliable, chunks are ordered strictly by frequency with equal frequencies
// being appended to maintain FIFO order.
//
// Parameters:
//   - a: the sweep being compared
//   - b: the reference sweep to compare against
//
// Returns:
//
//	1 if 'a' belongs after 'b' (higher frequency in same sweep or start of new sweep)
//	-1 if 'a' belongs before 'b' (part of previous sweep)
//	0 if either sweep is nil
func (sb *SweepsBuffer) compareSweepOrder(a, b *SweepResult) int {
	if a == nil || b == nil {
		return 0
	}

	ac := sb.getSweepOrder(a)
	bc := sb.getSweepOrder(b)

	rolloverThreshold := int((sb.maxFreq - sb.baseFreq) / sb.binWidth / 2)

	var ar, br bool
	if int((sb.maxFreq-sb.baseFreq)/sb.binWidth) > minSpectrumChunksThreshold {
		ar = ac < rolloverThreshold
		br = bc < rolloverThreshold
	}

	switch {
	case ar && br:
		if ac >= bc {
			return 1
		}
		return -1

	case ar:
		return 1

	case br:
		return -1

	default:
		if ac >= bc {
			return 1
		}
		return -1
	}
}

// updateFrequencyRange updates the buffer's frequency range based on the incoming sweep.
// During normal operation, the range will converge to the actual spectrum boundaries
// by tracking the minimum start frequency and maximum end frequency observed.
// The bin width is also updated from the sweep, which is expected to be constant
// across all sweeps in the spectrum.
//
// The function is safe to call multiple times as it will:
// - Only decrease baseFreq when a lower start frequency is seen
// - Only increase maxFreq when a higher end frequency is seen
// - Maintain consistent bin width across updates
func (sb *SweepsBuffer) updateFrequencyRange(s *SweepResult) {
	if s.StartFrequency < sb.baseFreq {
		sb.baseFreq = s.StartFrequency
	}
	if s.EndFrequency > sb.maxFreq {
		sb.maxFreq = s.EndFrequency
	}

	sb.binWidth = s.BinWidth
}
