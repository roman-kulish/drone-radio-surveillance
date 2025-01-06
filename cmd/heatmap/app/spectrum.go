package app

import (
	"math"
	"time"

	"github.com/roman-kulish/radio-surveillance/internal/spectrum"
)

type SpectrumData struct {
	Width, Height                int
	FrequencyMin, FrequencyMax   float64
	TimestampStart, TimestampEnd time.Time
	BoundsTracker                *SmoothBounds
	Spans                        [][]*float64
}

func NewSpectrumData(b *SmoothBounds) *SpectrumData {
	return &SpectrumData{
		Width:         0,
		Height:        0,
		FrequencyMin:  math.MaxFloat64,
		FrequencyMax:  0,
		BoundsTracker: b,
		Spans:         make([][]*float64, 0),
	}
}

func (s *SpectrumData) Update(span *spectrum.SpectralSpan[spectrum.SpectralPoint]) {
	s.Width = max(s.Width, len(span.Samples))
	s.Height++

	s.FrequencyMin = min(s.FrequencyMin, span.FrequencyStart)
	s.FrequencyMax = max(s.FrequencyMax, span.FrequencyEnd)

	if s.TimestampStart.IsZero() || s.TimestampStart.After(span.Timestamp) {
		s.TimestampStart = span.Timestamp
	}
	if s.TimestampEnd.IsZero() || s.TimestampEnd.Before(span.Timestamp) {
		s.TimestampEnd = span.Timestamp
	}

	powers := make([]*float64, len(span.Samples))
	for i, sample := range span.Samples {
		powers[i] = sample.Power
		s.BoundsTracker.Update(sample.Power)
	}
	s.Spans = append(s.Spans, powers)
}
