package entity

import (
	"github.com/Zyko0/EbitengineJam2026/core/entity/poses"
	"github.com/Zyko0/EbitengineJam2026/graphics"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/hajimehoshi/ebiten/v2"
)

type Context struct {
	PlayerPos     mgl64.Vec3
	ViewDir       mgl64.Vec3 // player look direction (normalized, includes pitch)
	Disconnected  bool       // blackout active: enemies that watch the player freeze
	PlayerRunning bool       // player sprinting: rouses the ceiling spider into a hunt
	World         World      // solid-cell lookups for line-of-sight tests
}

type Entity interface {
	Update(ctx *Context)
	Position() mgl64.Vec3
	Profile() *Profile
	SetAnim(a poses.Animation)
	CatchConfig() CatchConfig // grab parameters; zero Radius means it cannot catch
	SetCaught(v bool)         // freeze onto the strike anim while grabbing the player
	Caught() bool
	AppendGeometry(vx []ebiten.Vertex, ix []uint16, ctx *graphics.BillboardCtx) ([]ebiten.Vertex, []uint16)
}
