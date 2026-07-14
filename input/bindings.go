package input

import "github.com/hajimehoshi/ebiten/v2"

// Action is a device-independent game input. The tables below bind each action
// to every physical control that triggers it, so keyboard/mouse and gamepads
// drive the same actions and can be used at the same time.
type Action uint8

const (
	Run Action = iota
	Disconnect
	Pause
	Confirm
	Restart
	Quit
	actionCount
)

var keyBindings = [actionCount][]ebiten.Key{
	Run:        {ebiten.KeyShift},
	Disconnect: {ebiten.KeyF},
	Pause:      {ebiten.KeySpace},
	Confirm:    {ebiten.KeySpace},
	Restart:    {ebiten.KeyR},
	Quit:       {ebiten.KeyEscape},
}

// Standard layout, Xbox naming: A=RightBottom, B=RightRight, X=RightLeft,
// Y=RightTop. Quit stays keyboard-only; Start pauses instead.
var padBindings = [actionCount][]ebiten.StandardGamepadButton{
	Run: {
		ebiten.StandardGamepadButtonLeftStick,
		ebiten.StandardGamepadButtonFrontTopLeft,
	},
	Disconnect: {
		ebiten.StandardGamepadButtonRightRight,
		ebiten.StandardGamepadButtonFrontBottomRight,
	},
	Pause: {ebiten.StandardGamepadButtonCenterRight},
	Confirm: {
		ebiten.StandardGamepadButtonRightBottom,
		ebiten.StandardGamepadButtonCenterRight,
	},
	Restart: {
		ebiten.StandardGamepadButtonRightLeft,
		ebiten.StandardGamepadButtonCenterLeft,
	},
	Quit: nil,
}
