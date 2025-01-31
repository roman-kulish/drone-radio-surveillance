package sdr

import (
	"testing"
	"time"
)

func TestFrequencyBuffer_Ordering(t *testing.T) {
	// Create buffer with 1MHz to 6GHz range, capacity 10, flush 5
	fb, err := NewFrequencyBuffer(10, 5)
	if err != nil {
		t.Fatalf("Failed to create buffer: %v", err)
	}

	baseTime := time.Now()
	sweeps := []*SweepResult{
		{ // First sweep, first chunk
			StartFrequency: 5_000_700_000,
			EndFrequency:   5_000_800_000,
			BinWidth:       100_000,
			Timestamp:      baseTime,
		},
		{ // First sweep, first chunk
			StartFrequency: 5_000_600_000,
			EndFrequency:   5_000_700_000,
			BinWidth:       100_000,
			Timestamp:      baseTime,
		},
		{ // First sweep, first chunk
			StartFrequency: 5_000_800_000,
			EndFrequency:   5_000_900_000,
			BinWidth:       100_000,
			Timestamp:      baseTime,
		},
		{ // Second sweep starts
			StartFrequency: 1_000_000,
			EndFrequency:   1_100_000,
			BinWidth:       100_000,
			Timestamp:      baseTime.Add(time.Second),
		},
		{ // Should go before 6 GHz in first sweep
			StartFrequency: 5_000_900_000,
			EndFrequency:   5_001_000_000,
			BinWidth:       100_000,
			Timestamp:      baseTime.Add(2 * time.Second),
		},
		{ // Part of second sweep
			StartFrequency: 1_300_000,
			EndFrequency:   1_400_000,
			BinWidth:       100_000,
			Timestamp:      baseTime.Add(3 * time.Second),
		},
		{ // Part of second sweep
			StartFrequency: 1_200_000,
			EndFrequency:   1_300_000,
			BinWidth:       100_000,
			Timestamp:      baseTime.Add(3 * time.Second),
		},
	}

	// Insert all sweeps
	for i, sweep := range sweeps {
		err := fb.Insert(sweep)
		if err != nil {
			t.Errorf("Failed to insert sweep %d: %v", i, err)
		}
	}

	// Check buffer size
	if size := fb.Size(); size != len(sweeps) {
		t.Errorf("Expected buffer size %d, got %d", len(sweeps), size)
	}

	// Get all sweeps and verify order
	results := fb.Drain()
	if len(results) != len(sweeps) {
		t.Fatalf("Expected %d results, got %d", len(sweeps), len(results))
	}

	// Expected order of frequencies
	expected := []float64{
		5_000_600_000,
		5_000_700_000,
		5_000_800_000,
		5_000_900_000,
		1_000_000,
		1_200_000,
		1_300_000,
	}

	for i, freq := range expected {
		t.Logf("Result %d: expected frequency %.1f MHz, got %.1f MHz", i, freq/1e6, results[i].StartFrequency/1e6)

		if results[i].StartFrequency != freq {
			t.Errorf("Result %d: expected frequency %.1f MHz, got %.1f MHz", i, freq/1e6, results[i].StartFrequency/1e6)
		}
	}
}

func TestFrequencyBuffer_FlushBehavior(t *testing.T) {
	fb, err := NewFrequencyBuffer(3, 2)
	if err != nil {
		t.Fatalf("Failed to create buffer: %v", err)
	}

	baseTime := time.Now()
	sweeps := []*SweepResult{
		{
			StartFrequency: 5_000_600_000,
			EndFrequency:   5_000_800_000,
			BinWidth:       200_000,
			Timestamp:      baseTime,
		},
		{
			StartFrequency: 5_000_800_000,
			EndFrequency:   5_001_000_000,
			BinWidth:       200_000,
			Timestamp:      baseTime.Add(time.Second),
		},
		{
			StartFrequency: 1_000_000,
			EndFrequency:   1_200_000,
			BinWidth:       200_000,
			Timestamp:      baseTime.Add(2 * time.Second),
		},
	}

	// Insert until buffer is full
	for i, sweep := range sweeps {
		err := fb.Insert(sweep)
		if err != nil {
			t.Errorf("Failed to insert sweep %d: %v", i, err)
		}
	}

	// Verify buffer is full
	if !fb.IsFull() {
		t.Error("Buffer should be full")
	}

	// Flush and verify count
	flushed := fb.Flush()
	if len(flushed) != 2 {
		t.Errorf("Expected 2 flushed items, got %d", len(flushed))
	}

	// Verify remaining size
	if size := fb.Size(); size != 1 {
		t.Errorf("Expected remaining size 1, got %d", size)
	}

	// Verify frequencies of flushed items
	expected := []float64{
		5_000_600_000,
		5_000_800_000,
	}

	for i, freq := range expected {
		if flushed[i].StartFrequency != freq {
			t.Errorf("Flushed result %d: expected frequency %.1f MHz, got %.1f MHz",
				i, freq/1e6, flushed[i].StartFrequency/1e6)
		}
	}
}

func TestFrequencyBuffer_EdgeCases(t *testing.T) {
	fb, err := NewFrequencyBuffer(5, 2)
	if err != nil {
		t.Fatalf("Failed to create buffer: %v", err)
	}

	// Test nil sweep
	if err := fb.Insert(nil); err == nil {
		t.Error("Expected error when inserting nil sweep")
	}

	// Test empty buffer operations
	if fb.Flush() != nil {
		t.Error("Flush on empty buffer should return nil")
	}
	if fb.Drain() != nil {
		t.Error("Drain on empty buffer should return nil")
	}
	if fb.IsFull() {
		t.Error("Empty buffer should not be full")
	}
	if fb.Size() != 0 {
		t.Error("Empty buffer should have size 0")
	}

	// Test buffer creation with invalid parameters
	testCases := []struct {
		name     string
		capacity int
		flush    int
	}{
		{"invalid capacity", 0, 1},
		{"invalid flush count", 5, 6},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewFrequencyBuffer(tc.capacity, tc.flush)
			if err == nil {
				t.Error("Expected error for invalid parameters")
			}
		})
	}
}
