package ui

import (
	"image/color"
	"log"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
)

// Palette: black, white and a couple of greys, nothing else.
var (
	white = color.NRGBA{235, 235, 235, 255}
	black = color.NRGBA{0, 0, 0, 255}
	gray  = color.NRGBA{150, 150, 150, 255}
	wash  = color.NRGBA{0, 0, 0, 170} // full-screen dim behind overlay text
)

// Three sizes of the Go font cover every overlay: small for HUD labels and the
// info banner, medium for prompts and level info, large for the big titles.
var (
	faceSmall text.Face
	faceMed   text.Face
	faceBig   text.Face
)

func init() {
	src, err := opentype.Parse(goregular.TTF)
	if err != nil {
		log.Fatal("ui font: ", err)
	}
	newFace := func(size float64) text.Face {
		f, err := opentype.NewFace(src, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
		if err != nil {
			log.Fatal("ui face: ", err)
		}
		return text.NewGoXFace(f)
	}
	faceSmall = newFace(18)
	faceMed = newFace(30)
	faceBig = newFace(64)
}

// label draws one line anchored at (x,y) (y is the top of the line) with the
// given horizontal alignment and colour, scaled by alpha.
func label(dst *ebiten.Image, str string, face text.Face, x, y float64, align text.Align, clr color.Color, alpha float32) {
	op := &text.DrawOptions{}
	op.GeoM.Translate(x, y)
	op.PrimaryAlign = align
	op.SecondaryAlign = text.AlignStart
	op.ColorScale.ScaleWithColor(clr)
	op.ColorScale.ScaleAlpha(alpha)
	text.Draw(dst, str, face, op)
}

// outlined draws str with a black halo (eight offset copies) then the fill on
// top, so light text reads over any background.
func outlined(dst *ebiten.Image, str string, face text.Face, x, y float64, align text.Align, fill color.Color, alpha float32, thick float64) {
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			if dx == 0 && dy == 0 {
				continue
			}
			label(dst, str, face, x+float64(dx)*thick, y+float64(dy)*thick, align, black, alpha)
		}
	}
	label(dst, str, face, x, y, align, fill, alpha)
}

// overlayDim washes the whole frame toward black at the given strength, the
// backdrop for the victory and game-over text.
func overlayDim(dst *ebiten.Image, alpha float32) {
	c := wash
	c.A = uint8(float32(c.A) * clamp01(alpha))
	b := dst.Bounds()
	vector.FillRect(dst, 0, 0, float32(b.Dx()), float32(b.Dy()), c, false)
}

// meter draws a labelled bar: an outlined frame over a grey track with a white
// fill proportional to v in [0,1]. The label sits just above the bar.
func meter(dst *ebiten.Image, lbl string, x, y, w, h, v float64) {
	v = clamp01f(v)
	vector.FillRect(dst, float32(x), float32(y), float32(w), float32(h), gray, false)
	vector.FillRect(dst, float32(x), float32(y), float32(w*v), float32(h), white, false)
	vector.StrokeRect(dst, float32(x), float32(y), float32(w), float32(h), 2, black, false)
	outlined(dst, lbl, faceSmall, x, y-22, text.AlignStart, white, 1, 1)
}

// fadeIn ramps 0 to 1 over the first dur seconds since an overlay appeared.
func fadeIn(t, dur float64) float32 {
	if dur <= 0 || t >= dur {
		return 1
	}
	return float32(t / dur)
}

func clamp01(a float32) float32 {
	return max(0, min(1, a))
}

func clamp01f(v float64) float64 {
	return max(0, min(1, v))
}
