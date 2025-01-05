package app

import (
	_ "embed"
	"fmt"
	"image"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/golang/freetype"
	"golang.org/x/image/font"
)

//go:embed luxisr.ttf
var fontBytes []byte

const (
	dpi     float64 = 72
	hinting string  = "full"
	size    float64 = 18
	spacing float64 = 1.1
)

type Annotator struct {
	context *freetype.Context
}

func NewAnnotator() (*Annotator, error) {
	parsedFont, err := freetype.ParseFont(fontBytes)
	if err != nil {
		return nil, fmt.Errorf("parsing font: %w", err)
	}

	context := freetype.NewContext()
	context.SetDPI(dpi)
	context.SetFont(parsedFont)
	context.SetFontSize(size)
	context.SetSrc(image.White)

	switch hinting {
	case "full":
		context.SetHinting(font.HintingFull)
	default:
		context.SetHinting(font.HintingNone)
	}

	return &Annotator{context: context}, nil
}

func (a *Annotator) Annotate(img *image.RGBA, spec *SpectrumData) error {
	a.context.SetClip(img.Bounds())
	a.context.SetDst(img)

	ops := []struct {
		msg string
		fn  func(*image.RGBA, *SpectrumData) error
	}{
		{"drawing X scale", a.drawXScale},
		{"drawing Y scale", a.drawYScale},
		{"drawing info", a.drawInfo},
	}
	for _, op := range ops {
		if err := op.fn(img, spec); err != nil {
			return fmt.Errorf("%s: %w", op.msg, err)
		}
	}

	return nil
}

func (a *Annotator) drawXScale(img *image.RGBA, spec *SpectrumData) error {
	count := spec.Width / 350
	hzPerLabel := (spec.FrequencyMax - spec.FrequencyMin) / float64(count)
	pxPerLabel := spec.Width / count

	for si := 0; si < count; si++ {
		hz := spec.FrequencyMin + (float64(si) * hzPerLabel)
		px := si * pxPerLabel

		fract, suffix := humanize.ComputeSI(hz)
		str := fmt.Sprintf("%0.2f %sHz", fract, suffix)

		// draw a guideline on the exact frequency
		for i := 0; i < 30; i++ {
			img.Set(px, i, image.White)
		}

		// draw the text
		pt := freetype.Pt(px+5, 17)
		_, _ = a.context.DrawString(str, pt)
	}

	return nil
}

func (a *Annotator) drawYScale(img *image.RGBA, spec *SpectrumData) error {
	count := spec.Height / 100
	secsPerLabel := (spec.TimestampEnd.Unix() - spec.TimestampStart.Unix()) / int64(count)
	pxPerLabel := spec.Height / count

	for si := 0; si < count; si++ {
		secs := time.Duration(secsPerLabel*int64(si)) * time.Second
		px := si * pxPerLabel

		var str string
		if si == 0 {
			str = spec.TimestampStart.String()
		} else {
			point := spec.TimestampStart.Add(secs)
			str = point.Format("15:04:05")
		}

		// draw a guideline on the exact time
		for i := 0; i < 75; i++ {
			img.Set(i, px, image.White)
		}

		// draw the text, 3 px margin to the line
		pt := freetype.Pt(3, px-3)
		_, _ = a.context.DrawString(str, pt)
	}

	return nil
}

func (a *Annotator) drawInfo(img *image.RGBA, spec *SpectrumData) error {
	tPixel := (spec.TimestampEnd.Unix() - spec.TimestampStart.Unix()) / int64(spec.Height)

	fBandwidth := spec.FrequencyMax - spec.FrequencyMin
	fPixel := fBandwidth / float64(spec.Width)

	perPixel := fmt.Sprintf("%s x %d seconds", a.humanHz(fPixel), tPixel)

	// positioning
	imgSize := img.Bounds().Size()
	top, left := imgSize.Y-75, 3

	strings := []string{
		"Scan start: " + spec.TimestampStart.String(),
		"Scan end: " + spec.TimestampEnd.String(),
		fmt.Sprintf("Band: %s to %s", a.humanHz(spec.FrequencyMin), a.humanHz(spec.FrequencyMax)),
		fmt.Sprintf("Bandwidth: %s", a.humanHz(fBandwidth)),
		"1 pixel = " + perPixel,
	}

	// drawing
	pt := freetype.Pt(left, top)
	for _, s := range strings {
		_, _ = a.context.DrawString(s, pt)
		pt.Y += a.context.PointToFixed(size * spacing)
	}

	return nil
}

func (a *Annotator) humanHz(hz float64) string {
	fpxSI, fpxSuffix := humanize.ComputeSI(hz)
	return fmt.Sprintf("%0.2f %sHz", fpxSI, fpxSuffix)
}
