package input

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"
)

const (
	// Radial deadzone of the left (move) stick; speed ramps from zero at its edge.
	moveDeadzone = 0.25
	// Per-axis deadzone of the right (look) stick.
	lookDeadzone = 0.20
)

var gamepadIDs []ebiten.GamepadID

// pollGamepads merges buttons and sticks across every connected standard-layout
// gamepad, so any pad can drive the game. Move/look follow the same conventions
// as the keyboard/mouse paths: x strafes right, y walks forward, look up is
// positive pitch. active reports any pad input at all this tick.
func pollGamepads() (buttons [actionCount]bool, moveX, moveY, lookX, lookY float64, active bool) {
	gamepadIDs = ebiten.AppendGamepadIDs(gamepadIDs[:0])
	for _, id := range gamepadIDs {
		if !ebiten.IsStandardGamepadLayoutAvailable(id) {
			continue
		}
		for a := Action(0); a < actionCount; a++ {
			for _, b := range padBindings[a] {
				if ebiten.IsStandardGamepadButtonPressed(id, b) {
					buttons[a] = true
					active = true
				}
			}
		}

		// Dpad moves like the left stick, digitally.
		if ebiten.IsStandardGamepadButtonPressed(id, ebiten.StandardGamepadButtonLeftTop) {
			moveY++
		}
		if ebiten.IsStandardGamepadButtonPressed(id, ebiten.StandardGamepadButtonLeftBottom) {
			moveY--
		}
		if ebiten.IsStandardGamepadButtonPressed(id, ebiten.StandardGamepadButtonLeftRight) {
			moveX++
		}
		if ebiten.IsStandardGamepadButtonPressed(id, ebiten.StandardGamepadButtonLeftLeft) {
			moveX--
		}

		// Left stick: radial deadzone rescaled so walking speed ramps smoothly
		// from the deadzone edge to full deflection. Stick up reads -1.
		x := ebiten.StandardGamepadAxisValue(id, ebiten.StandardGamepadAxisLeftStickHorizontal)
		y := -ebiten.StandardGamepadAxisValue(id, ebiten.StandardGamepadAxisLeftStickVertical)
		if m := math.Hypot(x, y); m > moveDeadzone {
			s := min((m-moveDeadzone)/(1-moveDeadzone), 1) / m
			moveX += x * s
			moveY += y * s
		}

		// Right stick: squared response for fine aim near the center.
		lookX += lookCurve(ebiten.StandardGamepadAxisValue(id, ebiten.StandardGamepadAxisRightStickHorizontal))
		lookY += lookCurve(-ebiten.StandardGamepadAxisValue(id, ebiten.StandardGamepadAxisRightStickVertical))
	}
	active = active || moveX != 0 || moveY != 0 || lookX != 0 || lookY != 0

	return
}

// lookCurve applies the look deadzone and a squared response curve, keeping the
// sign of v.
func lookCurve(v float64) float64 {
	av := math.Abs(v)
	if av <= lookDeadzone {
		return 0
	}
	av = min((av-lookDeadzone)/(1-lookDeadzone), 1)

	return math.Copysign(av*av, v)
}
