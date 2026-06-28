package assets

import (
	_ "embed"
)

var (
	//go:embed audio/button_push.wav
	ButtonPushAudioBytes []byte

	//go:embed audio/elevator_leave.wav
	ElevatorLeaveAudioBytes []byte

	//go:embed audio/tallguy_laugh.wav
	TallguyLaughAudioBytes []byte

	//go:embed audio/hit.wav
	HitAudioBytes []byte
)
