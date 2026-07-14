package ui

import (
	"github.com/Zyko0/EbitengineJam2026/input"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

// DeathCause names how the run ended, picking the game-over subtitle.
type DeathCause int

const (
	DeathBySilhouette DeathCause = iota // the white silhouette closed the gap
	DeathByDamage                       // an enemy's grab drained the last heart
)

func (c DeathCause) subtitle() string {
	switch c {
	case DeathByDamage:
		return ""
	default:
		return "the white silhouette found you"
	}
}

// GameOver is the death overlay shown when the run ends. It washes the frame to
// near-black, names the death, and prompts a restart.
type GameOver struct {
	active bool
	cause  DeathCause
	t      float64 // seconds since shown, for the slow fade-in
}

func NewGameOver() *GameOver {
	return &GameOver{}
}

func (o *GameOver) Active() bool {
	return o.active
}

// Show raises the overlay for the given death cause. Idempotent while already up.
func (o *GameOver) Show(cause DeathCause) {
	if o.active {
		return
	}
	o.active = true
	o.cause = cause
	o.t = 0
}

func (o *GameOver) Hide() {
	o.active = false
}

func (o *GameOver) Update(dt float64) {
	if o.active {
		o.t += dt
	}
}

func (o *GameOver) Draw(dst *ebiten.Image) {
	if !o.active {
		return
	}
	a := fadeIn(o.t, 1.2) // lingering fade for the death beat
	overlayDim(dst, a)
	b := dst.Bounds()
	cx := float64(b.Dx()) / 2
	cy := float64(b.Dy()) / 2
	outlined(dst, "YOU DIED", faceBig, cx, cy-70, text.AlignCenter, white, a, 3)
	outlined(dst, o.cause.subtitle(), faceMed, cx, cy+20, text.AlignCenter, gray, a, 2)
	restart := "[ R ]  restart"
	if input.GamepadActive() {
		restart = "[ X ]  restart"
	}
	outlined(dst, restart, faceSmall, cx, cy+90, text.AlignCenter, white, a, 1)
}
