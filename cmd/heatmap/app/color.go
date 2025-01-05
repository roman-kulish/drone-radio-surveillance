package app

import (
	"image/color"
	"math"

	"github.com/lucasb-eyer/go-colorful"
)

const (
	hueStart = 236.0
	hueEnd   = 0.0
)

var noDataColor = color.Black

func pixelColor(power *float64, minPower, maxPower float64) color.Color {
	if power == nil {
		return noDataColor
	}

	span := (minPower - maxPower) * -1
	hPerDeg := (hueStart - hueEnd) / span

	powNormalized := *power - minPower
	powDegrees := powNormalized * hPerDeg

	hue := hueStart - powDegrees
	hue = math.Min(math.Max(hue, hueEnd), hueStart)

	return colorful.Hsv(hue, 1, 0.90)
}

// HSV represents a color in HSV color space
type HSV struct {
	H float64 // Hue [0-360]
	S float64 // Saturation [0-1]
	V float64 // Value [0-1]
}

// PowerToColor converts a normalized power value [0-1] to RGB color
// Uses an optimized "cold-to-hot" color scheme common in SDR applications
func PowerToColor(normalizedPower float64) color.Color {
	// Ensure input is in [0,1]
	power := math.Max(0, math.Min(1, normalizedPower))

	// Convert to HSV
	// Hue goes from 240 (blue) to 0 (red)
	// Higher power = "hotter" colors
	hsv := HSV{
		H: 240 - (power * 240),  // Blue->Red transition
		S: 0.9 + (power * 0.1),  // Slightly increase saturation with power
		V: math.Pow(power, 0.7), // Gamma correction for better visual perception
	}

	return HSVToRGB(hsv)
}

// PowerToColorEnhanced provides an enhanced color mapping with better
// differentiation in the lower power ranges
func PowerToColorEnhanced(normalizedPower float64) color.Color {
	// Ensure input is in [0,1]
	power := math.Max(0, math.Min(1, normalizedPower))

	// Apply non-linear transformation to enhance low-power visibility
	enhancedPower := math.Pow(power, 0.7)

	var hsv HSV

	// Multi-stage color mapping
	switch {
	case power < 0.25:
		// Black -> Blue transition
		hsv = HSV{
			H: 240,
			S: 1.0,
			V: enhancedPower * 4,
		}
	case power < 0.5:
		// Blue -> Cyan transition
		hsv = HSV{
			H: 240 - ((power - 0.25) * 240),
			S: 1.0,
			V: enhancedPower * 1.5,
		}
	case power < 0.75:
		// Cyan -> Yellow transition
		p := (power - 0.5) * 4
		hsv = HSV{
			H: 180 - (p * 120),
			S: 1.0,
			V: math.Min(1.0, enhancedPower*1.5),
		}
	default:
		// Yellow -> Red transition
		p := (power - 0.75) * 4
		hsv = HSV{
			H: 60 - (p * 60),
			S: 1.0,
			V: 1.0,
		}
	}

	return HSVToRGB(hsv)
}

// HSVToRGB converts HSV color space to RGB
// H: [0-360], S: [0-1], V: [0-1]
func HSVToRGB(hsv HSV) color.Color {
	h := hsv.H
	s := hsv.S
	v := hsv.V

	// Handle edge cases
	if s <= 0.0 {
		rgb := uint8(v * 255)
		return color.RGBA{R: rgb, G: rgb, B: rgb, A: 0xff}
	}

	// Normalize hue to [0-6]
	h = math.Mod(h, 360) / 60
	i := math.Floor(h)
	f := h - i

	p := v * (1 - s)
	q := v * (1 - s*f)
	t := v * (1 - s*(1-f))

	var r, g, b float64

	switch int(i) {
	case 0:
		r, g, b = v, t, p
	case 1:
		r, g, b = q, v, p
	case 2:
		r, g, b = p, v, t
	case 3:
		r, g, b = p, q, v
	case 4:
		r, g, b = t, p, v
	default:
		r, g, b = v, p, q
	}

	return color.RGBA{R: uint8(r * 255), G: uint8(g * 255), B: uint8(b * 255), A: 0xff}
}

// CreateColorMap generates a lookup table for quick color mapping
func CreateColorMap(size int, enhanced bool) []color.Color {
	colorMap := make([]color.Color, size)
	for i := 0; i < size; i++ {
		normalizedPower := float64(i) / float64(size-1)
		if enhanced {
			colorMap[i] = PowerToColorEnhanced(normalizedPower)
		} else {
			colorMap[i] = PowerToColor(normalizedPower)
		}
	}
	return colorMap
}
