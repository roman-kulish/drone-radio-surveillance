package app

import (
	"image/color"
	"math"
)

// ColorTheme represents a predefined color scheme for power visualization.
// Each theme is optimized for different visualization needs:
// - ClassicTheme: Traditional spectrum display (blue to red)
// - GrayscaleTheme: Monochrome visualization
// - JungleTheme: Nature-inspired colors for better contrast
// - ThermalTheme: Heat map visualization
// - MarineTheme: Water-depth inspired colors
type ColorTheme string

const (
	ClassicTheme   ColorTheme = "classic"   // Blue to red transition
	GrayscaleTheme ColorTheme = "grayscale" // Black to white transition
	JungleTheme    ColorTheme = "jungle"    // Dark green to yellow transition
	ThermalTheme   ColorTheme = "thermal"   // Black to red to yellow to white
	MarineTheme    ColorTheme = "marine"    // Deep blue to cyan to white

	DefaultColorMapSize = 256 // Default number of colors in the map
)

// ColorMapper provides efficient power-to-color mapping with support for
// different color themes and dynamic power range adjustment
type ColorMapper struct {
	colorMap      []color.Color // Pre-computed colors
	theme         func(float64) color.Color
	themeName     ColorTheme
	size          int     // Cache size
	powerPerIndex float64 // Power range per index step
	boundsMin     float64 // Cached bounds.Min
	boundsRange   float64 // Cached bounds.Max - bounds.Min
}

// NewColorMapper creates a new color mapper with specified theme and bounds.
// Uses default size (256) for the color map.
func NewColorMapper(theme ColorTheme, bounds PowerBounds) *ColorMapper {
	return NewColorMapperWithSize(theme, bounds, DefaultColorMapSize)
}

// NewColorMapperWithSize creates a new color mapper with specified size.
// Size determines the number of pre-computed colors in the map.
func NewColorMapperWithSize(theme ColorTheme, bounds PowerBounds, size int) *ColorMapper {
	if size <= 0 {
		size = DefaultColorMapSize
	}

	cm := &ColorMapper{
		colorMap:  make([]color.Color, size),
		theme:     getColorTheme(theme),
		themeName: theme,
		size:      size,
	}
	cm.UpdateBounds(bounds)
	return cm
}

// UpdateBounds updates the power bounds and recomputes the color map
func (cm *ColorMapper) UpdateBounds(bounds PowerBounds) {
	cm.boundsMin = bounds.Min
	cm.boundsRange = bounds.Max - bounds.Min
	cm.powerPerIndex = cm.boundsRange / float64(cm.size-1)

	// Rebuild color map
	for i := 0; i < cm.size; i++ {
		normalized := float64(i) / float64(cm.size-1)
		cm.colorMap[i] = cm.theme(normalized)
	}
}

// GetColor returns a color for the given power value
func (cm *ColorMapper) GetColor(power *float64) color.Color {
	if power == nil {
		return cm.colorMap[0] // Return min power color for invalid readings
	}

	// Convert power to index
	index := int((*power - cm.boundsMin) / cm.powerPerIndex)

	// Clamp index to valid range
	if index < 0 {
		return cm.colorMap[0]
	}
	if index >= cm.size {
		return cm.colorMap[cm.size-1]
	}
	return cm.colorMap[index]
}

// ThemeName returns the current color theme name
func (cm *ColorMapper) ThemeName() ColorTheme {
	return cm.themeName
}

// Size returns the color map size
func (cm *ColorMapper) Size() int {
	return cm.size
}

// HSV represents a color in HSV (Hue, Saturation, Value) color space
type HSV struct {
	H float64 // Hue angle in degrees [0-360]
	S float64 // Saturation [0-1]
	V float64 // Value/Brightness [0-1]
}

// RGB converts HSV to RGB color space efficiently
func (hsv HSV) RGB() color.Color {
	// Fast path for grayscale
	if hsv.S <= 0.0 {
		v := uint8(hsv.V * 255)
		return color.RGBA{R: v, G: v, B: v, A: 255}
	}

	// Normalize hue to [0-6)
	h := hsv.H
	if h >= 360 {
		h -= 360
	}
	h /= 60

	// Calculate color components
	i := int(h)
	f := h - float64(i)

	v := uint8(hsv.V * 255)
	p := uint8((hsv.V * (1 - hsv.S)) * 255)
	q := uint8((hsv.V * (1 - (hsv.S * f))) * 255)
	t := uint8((hsv.V * (1 - (hsv.S * (1 - f)))) * 255)

	switch i {
	case 0:
		return color.RGBA{R: v, G: t, B: p, A: 255}
	case 1:
		return color.RGBA{R: q, G: v, B: p, A: 255}
	case 2:
		return color.RGBA{R: p, G: v, B: t, A: 255}
	case 3:
		return color.RGBA{R: p, G: q, B: v, A: 255}
	case 4:
		return color.RGBA{R: t, G: p, B: v, A: 255}
	default: // case 5:
		return color.RGBA{R: v, G: p, B: q, A: 255}
	}
}

// Color theme implementations
func getColorTheme(theme ColorTheme) func(float64) color.Color {
	switch theme {
	case ClassicTheme:
		return func(power float64) color.Color {
			return HSV{
				H: 240 - (power * 240),
				S: 0.9 + (power * 0.1),
				V: math.Pow(power, 0.7),
			}.RGB()
		}

	case GrayscaleTheme:
		return func(power float64) color.Color {
			v := uint8(math.Pow(power, 0.7) * 255)
			return color.RGBA{R: v, G: v, B: v, A: 255}
		}

	case JungleTheme:
		return func(power float64) color.Color {
			return HSV{
				H: 120 - (power * 60),
				S: 1.0,
				V: 0.3 + (math.Pow(power, 0.6) * 0.7),
			}.RGB()
		}

	case ThermalTheme:
		return func(power float64) color.Color {
			if power < 0.33 {
				return color.RGBA{
					R: uint8((power * 3) * 255),
					A: 255,
				}
			}
			if power < 0.66 {
				return color.RGBA{
					R: 255,
					G: uint8(((power - 0.33) * 3) * 255),
					A: 255,
				}
			}
			return color.RGBA{
				R: 255,
				G: 255,
				B: uint8(((power - 0.66) * 3) * 255),
				A: 255,
			}
		}

	case MarineTheme:
		return func(power float64) color.Color {
			return HSV{
				H: 240 - (power * 60),
				S: 1.0 - (power * 0.8),
				V: 0.3 + (math.Pow(power, 0.6) * 0.7),
			}.RGB()
		}

	default: // Enhanced default theme
		return func(power float64) color.Color {
			power = math.Max(0, math.Min(1, power))
			enhanced := math.Pow(power, 0.7)

			switch {
			case power < 0.25:
				return HSV{
					H: 240,
					S: 1.0,
					V: enhanced * 4,
				}.RGB()
			case power < 0.5:
				return HSV{
					H: 240 - ((power - 0.25) * 240),
					S: 1.0,
					V: enhanced * 1.5,
				}.RGB()
			case power < 0.75:
				p := (power - 0.5) * 4
				return HSV{
					H: 180 - (p * 120),
					S: 1.0,
					V: math.Min(1.0, enhanced*1.5),
				}.RGB()
			default:
				p := (power - 0.75) * 4
				return HSV{
					H: 60 - (p * 60),
					S: 1.0,
					V: 1.0,
				}.RGB()
			}
		}
	}
}
