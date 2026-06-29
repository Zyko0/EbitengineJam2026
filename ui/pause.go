package ui

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

// Pause is the overlay menu raised mid-run with Space (and on wasm whenever the
// pointer lock is lost, e.g. the player presses Escape or defocuses the window).
// It dims the frame, lists the controls and ticks off the two run objectives as
// they are completed. The objective state is reset whenever the floor is cleared
// or the run restarts (see Game.regenerate / Game.restart).
type Pause struct {
	active    bool
	objButton bool // pressed the button that powers the exit elevator
	objEscape bool // reached the elevator and escaped the floor
}

func NewPause() *Pause {
	return &Pause{}
}

func (p *Pause) Active() bool {
	return p.active
}

// Toggle opens or closes the menu.
func (p *Pause) Toggle() {
	p.active = !p.active
}

// Show raises the menu (idempotent).
func (p *Pause) Show() {
	p.active = true
}

func (p *Pause) Hide() {
	p.active = false
}

// MarkButton / MarkEscape record objective completion for the checklist.
func (p *Pause) MarkButton() {
	p.objButton = true
}

func (p *Pause) MarkEscape() {
	p.objEscape = true
}

// ResetObjectives clears the checklist for a fresh floor or restart.
func (p *Pause) ResetObjectives() {
	p.objButton = false
	p.objEscape = false
}

func (p *Pause) Draw(dst *ebiten.Image) {
	if !p.active {
		return
	}
	overlayDim(dst, 1)
	b := dst.Bounds()
	cx := float64(b.Dx()) / 2
	cy := float64(b.Dy()) / 2
	lx := cx - 230 // left edge of the two columns

	outlined(dst, "PAUSE", faceBig, cx, cy-230, text.AlignCenter, white, 1, 3)

	// Controls.
	outlined(dst, "CONTROLS", faceMed, cx, cy-130, text.AlignCenter, gray, 1, 2)
	controls := [...][2]string{
		{"WASD", "Move"},
		{"Shift", "Run"},
		{"F", "Disconnect / Reconnect"},
		{"R", "Restart"},
		{"Space", "Pause / Resume"},
	}
	y := cy - 75
	for _, c := range controls {
		outlined(dst, c[0], faceSmall, lx, y, text.AlignStart, white, 1, 1)
		outlined(dst, c[1], faceSmall, lx+160, y, text.AlignStart, gray, 1, 1)
		y += 28
	}

	// Objectives.
	outlined(dst, "OBJECTIVES", faceMed, cx, cy+80, text.AlignCenter, gray, 1, 2)
	objs := [...]struct {
		done bool
		text string
	}{
		{p.objButton, "Find the button to power the elevator"},
		{p.objEscape, "Find the elevator and escape"},
	}
	y = cy + 135
	for _, o := range objs {
		mark, clr := "[  ]", gray
		if o.done {
			mark, clr = "[x]", white
		}
		outlined(dst, mark+"  "+o.text, faceSmall, lx, y, text.AlignStart, clr, 1, 1)
		y += 30
	}

	outlined(dst, "[ SPACE ]  resume", faceSmall, cx, cy+235, text.AlignCenter, white, 1, 1)
}
