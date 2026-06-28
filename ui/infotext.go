package ui

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

const (
	infoFade = 0.6 // seconds of fade in and of fade out
	infoHold = 3.0 // seconds held at full opacity between the fades
)

// InfoText is the transient banner high on the screen: a single short event
// message (a new hunter appeared, the exit powered on, the floor changed) that
// fades in, holds, then fades out. A new Push replaces whatever is showing.
type InfoText struct {
	msg   string
	timer float64 // seconds left in the fade-in / hold / fade-out cycle
}

func NewInfoText() *InfoText { return &InfoText{} }

// Push shows msg, restarting the full fade-in/hold/fade-out cycle.
func (i *InfoText) Push(msg string) {
	i.msg = msg
	i.timer = infoHold + 2*infoFade
}

// Update counts the banner's life down by dt seconds.
func (i *InfoText) Update(dt float64) {
	if i.timer > 0 {
		i.timer -= dt
	}
}

func (i *InfoText) Draw(dst *ebiten.Image) {
	if i.timer <= 0 || i.msg == "" {
		return
	}
	b := dst.Bounds()
	outlined(dst, i.msg, faceMed, float64(b.Dx())/2, float64(b.Dy())*0.16, text.AlignCenter, white, i.alpha(), 2)
}

// alpha ramps up over the first infoFade seconds and down over the last,
// otherwise full.
func (i *InfoText) alpha() float32 {
	switch {
	case i.timer >= infoHold+infoFade:
		return clamp01(float32((infoHold + 2*infoFade - i.timer) / infoFade))
	case i.timer <= infoFade:
		return clamp01(float32(i.timer / infoFade))
	default:
		return 1
	}
}
