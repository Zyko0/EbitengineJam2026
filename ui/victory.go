package ui

import (
	"fmt"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

type Victory struct {
	active bool
	floor  int
	t      float64 // seconds since shown, for the fade-in
}

func NewVictory() *Victory {
	return &Victory{}
}

func (v *Victory) Active() bool {
	return v.active
}

func (v *Victory) Show(floor int) {
	if v.active {
		return
	}
	v.active = true
	v.floor = floor
	v.t = 0
}

func (v *Victory) Hide() {
	v.active = false
}

func (v *Victory) Update(dt float64) {
	if v.active {
		v.t += dt
	}
}

func (v *Victory) Draw(dst *ebiten.Image) {
	if !v.active {
		return
	}
	a := fadeIn(v.t, 0.5)
	overlayDim(dst, a)
	b := dst.Bounds()
	cx := float64(b.Dx()) / 2
	cy := float64(b.Dy()) / 2
	outlined(dst, "FLOOR CLEARED", faceBig, cx, cy-130, text.AlignCenter, white, a, 3)
	outlined(dst, fmt.Sprintf("You escaped floor %d", v.floor), faceMed, cx, cy-30, text.AlignCenter, gray, a, 2)
	outlined(dst, "Ascend to the next floor?", faceMed, cx, cy+30, text.AlignCenter, white, a, 2)
	outlined(dst, "[ SPACE ]  continue", faceSmall, cx, cy+100, text.AlignCenter, white, a, 1)
}
