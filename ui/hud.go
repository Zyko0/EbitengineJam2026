package ui

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// Heart geometry: stretched horizontally so the hearts read wider than tall.
const (
	heartHalfH  = 11.0             // heart half-height
	heartHalfW  = heartHalfH * 1.3 // heart half-width
	heartPad    = 4.0              // room for the stroke + antialias
	heartStroke = 2.0              // outline width
	heartSpace  = 24.0             // distance between heart centres
	heartMargin = 28.0             // gap from the screen edges
)

// Both hearts are vector-drawn once into images at init; the HUD then just blits
// them. heartFilled is white with a black outline, heartEmpty is the outline
// only, for lost HP.
var (
	heartFilled *ebiten.Image
	heartEmpty  *ebiten.Image
)

func init() {
	w := int(math.Ceil(2 * (heartHalfW + heartPad)))
	h := int(math.Ceil(2 * (heartHalfH + heartPad)))
	cx, cy := float32(w)/2, float32(h)/2

	var p vector.Path
	p.MoveTo(cx, cy+0.95*heartHalfH) // bottom tip
	p.CubicTo(cx-0.6*heartHalfW, cy+0.35*heartHalfH, cx-1.0*heartHalfW, cy-0.35*heartHalfH, cx-0.5*heartHalfW, cy-0.7*heartHalfH)
	p.CubicTo(cx-0.2*heartHalfW, cy-0.95*heartHalfH, cx, cy-0.55*heartHalfH, cx, cy-0.3*heartHalfH) // top dip
	p.CubicTo(cx, cy-0.55*heartHalfH, cx+0.2*heartHalfW, cy-0.95*heartHalfH, cx+0.5*heartHalfW, cy-0.7*heartHalfH)
	p.CubicTo(cx+1.0*heartHalfW, cy-0.35*heartHalfH, cx+0.6*heartHalfW, cy+0.35*heartHalfH, cx, cy+0.95*heartHalfH)
	p.Close()

	stroke := func(dst *ebiten.Image) {
		op := &vector.DrawPathOptions{AntiAlias: true}
		op.ColorScale.ScaleWithColor(black)
		vector.StrokePath(dst, &p, &vector.StrokeOptions{Width: heartStroke, LineJoin: vector.LineJoinRound}, op)
	}

	heartFilled = ebiten.NewImage(w, h)
	fop := &vector.DrawPathOptions{AntiAlias: true}
	fop.ColorScale.ScaleWithColor(white)
	vector.FillPath(heartFilled, &p, &vector.FillOptions{}, fop)
	stroke(heartFilled)

	heartEmpty = ebiten.NewImage(w, h)
	stroke(heartEmpty)
}

type HUD struct{}

func NewHUD() *HUD {
	return &HUD{}
}

// Draw renders the meters in the lower-left corner and the HP hearts in the
// upper-right. charge and silhouette are both in [0,1]; values outside are
// clamped by meter. hp is the current health out of maxHP.
func (h *HUD) Draw(dst *ebiten.Image, charge, silhouette float64, hp, maxHP int) {
	const (
		barW = 220.0
		barH = 16.0
		x    = 28.0
		gap  = 44.0
	)

	y := float64(dst.Bounds().Dy()) - 96.0
	meter(dst, "WHITE SILHOUETTE", x, y, barW, barH, silhouette)
	meter(dst, "CHARGE", x, y+gap, barW, barH, charge)

	hearts(dst, hp, maxHP)
}

// hearts blits maxHP heart slots in the upper-right corner: filled for remaining
// HP, empty for lost HP. HP drains from the right.
func hearts(dst *ebiten.Image, hp, maxHP int) {
	iw := float64(heartFilled.Bounds().Dx())
	ih := float64(heartFilled.Bounds().Dy())
	cy := heartMargin + ih/2
	rightCx := float64(dst.Bounds().Dx()) - heartMargin - iw/2
	for i := range maxHP {
		img := heartEmpty
		if i < hp {
			img = heartFilled
		}
		cx := rightCx - float64(maxHP-1-i)*heartSpace
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(cx-iw/2, cy-ih/2)
		dst.DrawImage(img, op)
	}
}
