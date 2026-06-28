package entity

import (
	xaudio "github.com/Zyko0/EbitengineJam2026/audio"
	"github.com/Zyko0/EbitengineJam2026/core/entity/poses"
	"github.com/go-gl/mathgl/mgl64"
)

const patrollerWallSpeed = 0.08

// newPatrollerVoice claims the shared looping gibberish player for the given
// timbre (one instance per preset, reused across levels), places it at pos so it
// attenuates from frame one (otherwise it sits on the listener at the origin and
// blares until the first Update), and starts it; SetPosition it each frame after.
func newPatrollerVoice(pos mgl64.Vec3, voice xaudio.Voice) *xaudio.SpatialPlayer {
	v := xaudio.PatrollerVoice(voice)
	v.SetPosition([3]float64(pos))
	v.Play()
	return v
}

type PatrollerWall struct {
	*baseEntity
	path  []mgl64.Vec3
	idx   int
	voice *xaudio.SpatialPlayer
}

func NewPatrollerWall(pos mgl64.Vec3, circuit []mgl64.Vec3) *PatrollerWall {
	mid := len(circuit) / 2
	pw := &PatrollerWall{
		baseEntity: newBaseEntity(circuit[mid], Humanoid),
		path:       circuit,
		idx:        (mid + 1) % len(circuit),
		voice:      newPatrollerVoice(circuit[mid], xaudio.VoiceDeep),
	}
	pw.SetAnim(poses.WalkingAnim)

	return pw
}

func (p *PatrollerWall) Update(ctx *Context) {
	p.voice.SetPosition([3]float64(p.pos)) // keep the whisper tracking its position

	// While grabbing the player it holds its ground and plays the strike.
	if p.Caught() {
		p.baseEntity.update(ctx)
		return
	}

	// Movement update
	target := p.path[p.idx]
	d := mgl64.Vec3{
		target.X() - p.pos.X(),
		0,
		target.Z() - p.pos.Z(),
	}
	dist := d.Len()
	if dist <= patrollerWallSpeed {
		p.pos = mgl64.Vec3{target.X(), p.pos.Y(), target.Z()}
		p.idx = (p.idx + 1) % len(p.path)
		return
	}
	p.pos = p.pos.Add(d.Mul(patrollerWallSpeed / dist))

	// Base update
	p.baseEntity.update(ctx)
}
