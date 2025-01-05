package app

import (
	"image/color"
	"math"
	"sync"
)

const (
	ClassicTheme   ColorTheme = "classic"
	GrayscaleTheme ColorTheme = "grayscale"
	JungleTheme    ColorTheme = "jungle"
	ThermalTheme   ColorTheme = "thermal"
	MarineTheme    ColorTheme = "marine"
)

type ColorTheme string

var InvalidPowerColor = color.Black

// Improved version with better bounds integration:
type ColorMapper struct {
	colorMap      []color.Color // Pre-computed colors
	bounds        PowerBounds
	theme         func(float64) color.Color
	size          int     // Cache size
	powerPerIndex float64 // Power range per index step
	mu            sync.RWMutex
}

func NewColorMapper(size int, theme ColorTheme, bounds PowerBounds) *ColorMapper {
	cm := &ColorMapper{
		colorMap: make([]color.Color, size),
		theme:    GetColorTheme(theme),
		size:     size,
	}

	// Initialize with default bounds
	cm.UpdateBounds(bounds)
	return cm
}

func (cm *ColorMapper) UpdateBounds(bounds PowerBounds) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Update bounds
	cm.bounds = bounds
	cm.powerPerIndex = (cm.bounds.Max - cm.bounds.Min) / float64(cm.size-1)

	// Rebuild color map
	for i := 0; i < cm.size; i++ {
		// Convert index directly to power value
		power := cm.bounds.Min + (float64(i) * cm.powerPerIndex)

		// Normalize power to [0,1]
		normalized := (power - cm.bounds.Min) / (cm.bounds.Max - cm.bounds.Min)

		// Get color from theme
		cm.colorMap[i] = cm.theme(normalized)
	}
}

// More efficient power to color mapping
func (cm *ColorMapper) GetColor(power *float64) color.Color {
	if power == nil {
		return InvalidPowerColor
	}

	pwr := *power

	cm.mu.RLock()
	defer cm.mu.RUnlock()

	pwr = math.Max(cm.bounds.Min, math.Min(pwr, cm.bounds.Max))

	// Direct conversion to index using pre-calculated values
	index := int((pwr - cm.bounds.Min) / cm.powerPerIndex)

	// Safety clamp (shouldn't be needed but good practice)
	if index < 0 {
		index = 0
	} else if index >= cm.size {
		index = cm.size - 1
	}

	return cm.colorMap[index]
}

// HSV represents a color in HSV color space
type HSV struct {
	H float64 // Hue [0-360]
	S float64 // Saturation [0-1]
	V float64 // Value [0-1]
}

// RGB converts HSV color space to RGB
// H: [0-360], S: [0-1], V: [0-1]
func (hsv HSV) RGB() color.Color {
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

	return hsv.RGB()
}

// GetColorTheme returns predefined color themes
func GetColorTheme(theme ColorTheme) func(float64) color.Color {
	switch theme {
	case ClassicTheme: // Blue -> Red
		return func(power float64) color.Color {
			hsv := HSV{
				H: 240 - (power * 240),
				S: 0.9 + (power * 0.1),
				V: math.Pow(power, 0.7),
			}
			return hsv.RGB()
		}

	case GrayscaleTheme: // Black -> White
		return func(power float64) color.Color {
			v := math.Pow(power, 0.7) * 255
			return color.RGBA{R: uint8(v), G: uint8(v), B: uint8(v), A: 0xff}
		}

	case JungleTheme: // Dark Green -> Yellow
		return func(power float64) color.Color {
			hsv := HSV{
				H: 120 - (power * 60),
				S: 1.0,
				V: 0.3 + (math.Pow(power, 0.6) * 0.7),
			}
			return hsv.RGB()
		}

	case ThermalTheme: // Black -> Red -> Yellow -> White
		return func(power float64) color.Color {
			if power < 0.33 {
				p := power * 3
				return color.RGBA{R: uint8(p * 255), A: 0xff}
			} else if power < 0.66 {
				p := (power - 0.33) * 3
				return color.RGBA{R: 255, G: uint8(p * 255), A: 0xff}
			}
			p := (power - 0.66) * 3
			v := uint8(p * 255)
			return color.RGBA{R: 255, G: 255, B: v, A: 0xff}
		}

	case MarineTheme: // Deep Blue -> Cyan -> White
		return func(power float64) color.Color {
			hsv := HSV{
				H: 240 - (power * 60),
				S: 1.0 - (power * 0.8),
				V: 0.3 + (math.Pow(power, 0.6) * 0.7),
			}
			return hsv.RGB()
		}

	default: // Enhanced default
		return PowerToColorEnhanced
	}
}
