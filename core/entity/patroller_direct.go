package entity

import (
	xaudio "github.com/Zyko0/EbitengineJam2026/audio"
	"github.com/Zyko0/EbitengineJam2026/core/entity/poses"
	"github.com/go-gl/mathgl/mgl64"
)

const patrollerDirectSpeed = 0.08

type PatrollerDirect struct {
	*baseEntity
	path  []mgl64.Vec3
	idx   int
	voice *xaudio.SpatialPlayer
}

func NewPatrollerDirect(pos mgl64.Vec3, circuit []mgl64.Vec3) *PatrollerDirect {
	pd := &PatrollerDirect{
		baseEntity: newBaseEntity(pos, Humanoid),
		path:       circuit,
		idx:        1,
		voice:      newPatrollerVoice(pos, xaudio.VoiceLight),
	}
	pd.SetAnim(poses.CatwalkWalkingAnim)

	return pd
}

func (p *PatrollerDirect) Update(ctx *Context) {
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
	if dist <= patrollerDirectSpeed {
		p.pos = mgl64.Vec3{target.X(), p.pos.Y(), target.Z()}
		p.idx = (p.idx + 1) % len(p.path)
		return
	}
	p.pos = p.pos.Add(d.Mul(patrollerDirectSpeed / dist))

	// Base update
	p.baseEntity.update(ctx)
}
