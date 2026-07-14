package input

import (
	"math"
	"runtime"

	"github.com/Zyko0/EbitengineJam2026/logic"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/hajimehoshi/ebiten/v2"
)

// Gamepad look speed at full stick deflection, radians per second.
const (
	padYawSpeed   = 3.0
	padPitchSpeed = 2.0
)

var (
	inputGiven bool
	cursorInit bool

	lcx, lcy int

	pressed, prevPressed [actionCount]bool

	moveX, moveY       float64 // strafe (+right) / walk (+forward), |v| <= 1
	lookYaw, lookPitch float64 // this tick's camera deltas, radians

	padActive bool // a gamepad was the last device used
)

// Update polls every device once per tick; call it before anything reads input.
// All queries below report the state captured here, including the just-pressed
// edges, so no caller ever needs to drain deltas itself.
func Update() {
	prevPressed = pressed

	padButtons, pmx, pmy, plx, ply, padUsed := pollGamepads()

	kbUsed := false
	for a := Action(0); a < actionCount; a++ {
		kb := false
		for _, k := range keyBindings[a] {
			if ebiten.IsKeyPressed(k) {
				kb = true
				break
			}
		}
		kbUsed = kbUsed || kb
		pressed[a] = kb || padButtons[a]
	}

	// Move: WASD plus the pad's contribution, clamped to unit length so
	// diagonals and dual-device input never exceed full speed.
	var kx, ky float64
	if ebiten.IsKeyPressed(ebiten.KeyW) {
		ky++
	}
	if ebiten.IsKeyPressed(ebiten.KeyS) {
		ky--
	}
	if ebiten.IsKeyPressed(ebiten.KeyD) {
		kx++
	}
	if ebiten.IsKeyPressed(ebiten.KeyA) {
		kx--
	}
	kbUsed = kbUsed || kx != 0 || ky != 0
	moveX, moveY = kx+pmx, ky+pmy
	if m := math.Hypot(moveX, moveY); m > 1 {
		moveX, moveY = moveX/m, moveY/m
	}

	// Look: mouse deltas while the cursor is captured, plus the right stick.
	mx, my := mouseDelta()
	sens := logic.MouseSensitivity / 1000
	lookYaw = mx*sens + plx*padYawSpeed/logic.TPS
	lookPitch = my*sens + ply*padPitchSpeed/logic.TPS

	// Last-device tracking, for UI prompts and the wasm start gate. Mouse motion
	// only counts while captured, so hovering an unlocked cursor over the window
	// never yanks control away from a pad player.
	captured := ebiten.CursorMode() == ebiten.CursorModeCaptured
	switch {
	case padUsed:
		padActive = true
	case kbUsed, ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft),
		captured && (mx != 0 || my != 0):
		padActive = false
	}
}

// Pressed reports the action is held this tick, on any device.
func Pressed(a Action) bool {
	return pressed[a]
}

// JustPressed reports the action went from released to held this tick.
func JustPressed(a Action) bool {
	return pressed[a] && !prevPressed[a]
}

// Move returns the movement intent: x strafes right, y walks forward.
// Magnitude is at most 1; keyboard diagonals normalize like analog input.
func Move() (x, y float64) {
	return moveX, moveY
}

// Moving reports any movement intent this tick.
func Moving() bool {
	return moveX != 0 || moveY != 0
}

// Look returns this tick's camera yaw/pitch deltas in radians, mouse and right
// stick combined, sensitivity applied.
func Look() (yaw, pitch float64) {
	return lookYaw, lookPitch
}

// GamepadActive reports a gamepad was the device used most recently: UI prompts
// switch to pad labels and the wasm start gate accepts pad play without a
// pointer lock.
func GamepadActive() bool {
	return padActive
}

// ProcessMovement advances pos by this tick's movement intent at speed ms,
// walking along dir and strafing along right (both flattened to the XZ plane).
func ProcessMovement(pos, dir, right mgl64.Vec3, ms float64) mgl64.Vec3 {
	if moveX == 0 && moveY == 0 {
		return pos
	}
	d := mgl64.Vec2{dir.X(), dir.Z()}.Normalize().Mul(ms * moveY)
	r := mgl64.Vec2{right.X(), right.Z()}.Normalize().Mul(ms * moveX)

	return pos.Add(mgl64.Vec3{d.X() + r.X(), 0, d.Y() + r.Y()})
}

func EnsureCursorCaptured() bool {
	if ebiten.CursorMode() == ebiten.CursorModeCaptured {
		return true
	}

	inputGiven = inputGiven || ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft)
	if runtime.GOOS != "js" || inputGiven {
		ebiten.SetCursorMode(ebiten.CursorModeCaptured)
		inputGiven = false
	}

	return false
}

// mouseDelta returns the cursor movement since last tick, in pixels, y up
// positive. It reads zero while the cursor is not captured so an unlocked
// pointer (wasm pause, gamepad play) never turns the camera.
func mouseDelta() (float64, float64) {
	var xoff, yoff int
	cx, cy := ebiten.CursorPosition()
	// Note: ebitengine hack, with mouse captured initial cursor position is wrong
	if !cursorInit {
		if cx != 0 && cy != 0 {
			cursorInit = true
		}
	} else if cx != lcx || cy != lcy {
		xoff, yoff = cx-lcx, lcy-cy
	}
	lcx, lcy = cx, cy

	if ebiten.CursorMode() != ebiten.CursorModeCaptured {
		return 0, 0
	}

	return float64(xoff), float64(yoff)
}
