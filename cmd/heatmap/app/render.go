package app

import (
	_ "embed"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"strings"
	"time"

	"github.com/golang/freetype"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"
)

//go:embed RobotoMono-Regular.ttf
var fontBytes []byte

const (
	dpi            = 120.0
	fontSize       = 12.0
	tickMarkHeight = 5
	pixelsPerLabel = 150.00

	// Default border sizes in pixels
	defaultTopBorder    = 40
	defaultLeftBorder   = 80
	defaultBottomBorder = 40
	defaultRightBorder  = 40

	defaultTimeFormat     = "15:04"
	defaultDatetimeFormat = time.DateTime
)

// BorderConfig defines the sizes of white space around the spectrum
type BorderConfig struct {
	Top    int // Space for frequency scale
	Left   int // Space for time scale
	Bottom int // Space for information bar
	Right  int // Right padding
}

// RenderConfig holds all configuration options for spectrum visualization
type RenderConfig struct {
	// Time display configuration
	TimeFormat     string         // Format string for time display (e.g. "15:04")
	DatetimeFormat string         // Format string for date/time display
	Location       *time.Location // Timezone for time display

	// Visual configuration
	FontSize     float64    // Font size in points
	ColorTheme   ColorTheme // Color scheme for power values
	ColorMapSize int        // Number of colors in gradient (0 for default)

	// Border configuration
	BorderConfig BorderConfig
}

// SpectrumRenderer handles the visualization of radio spectrum data
type SpectrumRenderer struct {
	colorMap *ColorMapper
	config   RenderConfig
}

// NewSpectrumRenderer creates a new spectrum renderer with the given configuration
func NewSpectrumRenderer(config RenderConfig) (*SpectrumRenderer, error) {
	// Set defaults for zero values
	if config.TimeFormat == "" {
		config.TimeFormat = defaultTimeFormat
	}
	if config.DatetimeFormat == "" {
		config.DatetimeFormat = defaultDatetimeFormat
	}
	if config.Location == nil {
		config.Location = time.Local
	}
	if config.FontSize == 0 {
		config.FontSize = fontSize
	}
	if config.BorderConfig.Top == 0 {
		config.BorderConfig.Top = defaultTopBorder
	}
	if config.BorderConfig.Left == 0 {
		config.BorderConfig.Left = defaultLeftBorder
	}
	if config.BorderConfig.Bottom == 0 {
		config.BorderConfig.Bottom = defaultBottomBorder
	}
	if config.BorderConfig.Right == 0 {
		config.BorderConfig.Right = defaultRightBorder
	}

	return &SpectrumRenderer{config: config}, nil
}

// Render creates an image of the spectrum data with annotations
func (r *SpectrumRenderer) Render(spec *SpectrumData) (*image.RGBA, error) {
	// Create image with space for borders
	fullWidth := spec.Width + r.config.BorderConfig.Left + r.config.BorderConfig.Right
	fullHeight := spec.Height + r.config.BorderConfig.Top + r.config.BorderConfig.Bottom
	img := image.NewRGBA(image.Rect(0, 0, fullWidth, fullHeight))

	// Fill with white background
	draw.Draw(img, img.Bounds(), image.White, image.Point{}, draw.Src)

	// Define spectrum area (1:1 mapping)
	spectrumArea := image.Rect(
		r.config.BorderConfig.Left,
		r.config.BorderConfig.Top,
		r.config.BorderConfig.Left+spec.Width,
		r.config.BorderConfig.Top+spec.Height,
	)

	// Update or create color map
	bounds := spec.BoundsTracker.Current()
	if r.colorMap == nil {
		r.colorMap = NewColorMapper(r.config.ColorTheme, bounds)
	} else {
		r.colorMap.UpdateBounds(bounds)
	}

	// Create annotator for drawing scales and labels
	ann, err := newAnnotator(annotatorConfig{
		TimeFormat:     r.config.TimeFormat,
		DatetimeFormat: r.config.DatetimeFormat,
		Location:       r.config.Location,
		FontSize:       r.config.FontSize,
		Borders:        r.config.BorderConfig,
	})
	if err != nil {
		return nil, fmt.Errorf("creating annotator: %w", err)
	}
	defer ann.Close()

	// First draw annotations
	if err = ann.annotate(img, spec); err != nil {
		return nil, fmt.Errorf("drawing annotations: %w", err)
	}

	// Then render spectrum data (overwriting any overlapping annotations)
	r.renderSpectrum(img, spectrumArea, spec)

	return img, nil
}

// renderSpectrum draws the actual spectrum data using the color map
func (r *SpectrumRenderer) renderSpectrum(img *image.RGBA, area image.Rectangle, spec *SpectrumData) {
	for y, span := range spec.Spans {
		imgY := area.Min.Y + y
		for x, power := range span {
			imgX := area.Min.X + x
			if power != nil {
				img.Set(imgX, imgY, r.colorMap.GetColor(power))
			}
		}
	}
}

// Internal annotator implementation
type annotatorConfig struct {
	TimeFormat     string
	DatetimeFormat string
	Location       *time.Location
	FontSize       float64
	Borders        BorderConfig
}

type annotator struct {
	context  *freetype.Context
	config   annotatorConfig
	fontFace font.Face
}

func newAnnotator(config annotatorConfig) (*annotator, error) {
	parsedFont, err := freetype.ParseFont(fontBytes)
	if err != nil {
		return nil, fmt.Errorf("parsing font: %w", err)
	}

	ctx := freetype.NewContext()
	ctx.SetDPI(dpi)
	ctx.SetFont(parsedFont)
	ctx.SetFontSize(config.FontSize)
	ctx.SetHinting(font.HintingNone)
	ctx.SetSrc(image.Black)

	return &annotator{
		context: ctx,
		config:  config,
		fontFace: truetype.NewFace(parsedFont, &truetype.Options{
			Size:    config.FontSize,
			DPI:     dpi,
			Hinting: font.HintingNone,
		}),
	}, nil
}

func (a *annotator) Close() error {
	if a.fontFace != nil {
		return a.fontFace.Close()
	}
	return nil
}

func (a *annotator) annotate(img *image.RGBA, spec *SpectrumData) error {
	a.context.SetClip(img.Bounds())
	a.context.SetDst(img)

	if err := a.drawFrequencyScale(img, spec); err != nil {
		return fmt.Errorf("drawing frequency scale: %w", err)
	}
	if err := a.drawTimeScale(img, spec); err != nil {
		return fmt.Errorf("drawing time scale: %w", err)
	}
	if err := a.drawInfoBar(img, spec); err != nil {
		return fmt.Errorf("drawing info bar: %w", err)
	}

	return nil
}

func (a *annotator) drawFrequencyScale(img *image.RGBA, spec *SpectrumData) error {
	freqStep := calculateNiceFrequencyStep(spec.FrequencyMax-spec.FrequencyMin, spec.Width)
	startFreq := math.Floor(spec.FrequencyMin/freqStep) * freqStep

	// Get actual font height in pixels
	metrics := a.fontFace.Metrics()
	fontHeight := (metrics.Ascent + metrics.Descent).Round()

	// Calculate centered Y position in the available space (35px)
	textY := a.config.Borders.Top - fontHeight/2

	for freq := startFreq; freq <= spec.FrequencyMax; freq += freqStep {
		// Convert frequency to x coordinate
		xRatio := (freq - spec.FrequencyMin) / (spec.FrequencyMax - spec.FrequencyMin)
		x := a.config.Borders.Left + int(xRatio*float64(spec.Width))

		// Draw tick mark
		for y := a.config.Borders.Top - tickMarkHeight; y < a.config.Borders.Top; y++ {
			img.Set(x, y, color.Black)
		}

		// Format and draw frequency label
		label := formatFrequency(freq)
		width := font.MeasureString(a.fontFace, label)
		pt := freetype.Pt(x-(width.Round()/2), textY)
		_, err := a.context.DrawString(label, pt)
		if err != nil {
			return fmt.Errorf("drawing frequency label: %w", err)
		}
	}
	return nil
}

func (a *annotator) drawTimeScale(img *image.RGBA, spec *SpectrumData) error {
	duration := spec.TimestampEnd.Sub(spec.TimestampStart)
	timeStep := calculateNiceTimeStep(duration)

	// Get font metrics once
	metrics := a.fontFace.Metrics()
	fontHeight := (metrics.Ascent + metrics.Descent).Round()

	currentTime := spec.TimestampStart
	for y := 0; y < spec.Height; y += int(timeStep.Seconds()) {
		imgY := y + a.config.Borders.Top

		// Draw tick mark
		for x := a.config.Borders.Left - 5; x < a.config.Borders.Left; x++ {
			img.Set(x, imgY, color.Black)
		}

		// Center text vertically relative to the tick mark position
		textY := imgY + fontHeight/2 - metrics.Descent.Round()

		// Format and draw time label
		timeInLoc := currentTime.In(a.config.Location)
		label := timeInLoc.Format(a.config.TimeFormat)
		pt := freetype.Pt(10, textY)
		_, err := a.context.DrawString(label, pt)
		if err != nil {
			return fmt.Errorf("drawing time label: %w", err)
		}

		currentTime = currentTime.Add(timeStep)
	}
	return nil
}

func (a *annotator) drawInfoBar(img *image.RGBA, spec *SpectrumData) error {
	var sb strings.Builder

	sb.WriteString(formatFrequencyRange(spec.FrequencyMin, spec.FrequencyMax))
	sb.WriteString("; ")
	sb.WriteString(fmt.Sprintf("Time: %s - %s",
		spec.TimestampStart.In(a.config.Location).Format(a.config.DatetimeFormat),
		spec.TimestampEnd.In(a.config.Location).Format(a.config.DatetimeFormat)))

	// Calculate pixel resolution in frequency
	freqPerPixel := (spec.FrequencyMax - spec.FrequencyMin) / float64(spec.Width)

	sb.WriteString("; ")
	sb.WriteString(fmt.Sprintf("1px = %s", formatFrequency(freqPerPixel)))

	// Calculate text position in bottom border
	metrics := a.fontFace.Metrics()
	fontHeight := (metrics.Ascent + metrics.Descent).Round()

	// Center text vertically in bottom border
	textY := img.Bounds().Max.Y - (a.config.Borders.Bottom-fontHeight)/2 - metrics.Descent.Round()

	// Draw info
	pt := freetype.Pt(a.config.Borders.Left, textY)
	_, err := a.context.DrawString(sb.String(), pt)
	if err != nil {
		return fmt.Errorf("drawing info text: %w", err)
	}

	return nil
}

// Helper functions

func calculateNiceFrequencyStep(range_ float64, width int) float64 {
	// Standard step sizes in Hz
	steps := []float64{
		1,             // 1 Hz
		10,            // 10 Hz
		100,           // 100 Hz
		1_000,         // 1 kHz
		10_000,        // 10 kHz
		100_000,       // 100 kHz
		1_000_000,     // 1 MHz
		10_000_000,    // 10 MHz
		100_000_000,   // 100 MHz
		1_000_000_000, // 1 GHz
	}

	desiredSteps := float64(width) / pixelsPerLabel
	targetStep := range_ / desiredSteps

	// Find the closest standard step size
	for _, step := range steps {
		if step >= targetStep {
			// If this step would give us at least 2 points
			if range_/step >= 2 {
				return step
			}
			break
		}
	}

	// If we can't find a suitable step or would get too few points,
	// return half the range to show at least center frequency
	return range_ / 2
}

func formatFrequency(freq float64) string {
	switch {
	case freq >= 1e9:
		return fmt.Sprintf("%.1f GHz", freq/1e9)
	case freq >= 1e6:
		return fmt.Sprintf("%.1f MHz", freq/1e6)
	case freq >= 1e3:
		return fmt.Sprintf("%.1f kHz", freq/1e3)
	default:
		return fmt.Sprintf("%.0f Hz", freq)
	}
}

func formatFrequencyRange(min, max float64) string {
	return fmt.Sprintf("Freq: %s - %s", formatFrequency(min), formatFrequency(max))
}

func calculateNiceTimeStep(duration time.Duration) time.Duration {
	seconds := duration.Seconds()
	roughStep := seconds / 8 // Aim for about 8 time labels

	// Nice time intervals in seconds
	niceIntervals := []float64{
		60,    // 1 minute
		300,   // 5 minutes
		600,   // 10 minutes
		900,   // 15 minutes
		1800,  // 30 minutes
		3600,  // 1 hour
		7200,  // 2 hours
		14400, // 4 hours
	}

	// Find the first interval larger than our rough step
	for _, interval := range niceIntervals {
		if roughStep <= interval {
			return time.Duration(interval) * time.Second
		}
	}

	return time.Hour * 6 // Default for very long durations
}
