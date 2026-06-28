package core

import (
	"math"

	"github.com/Zyko0/EbitengineJam2026/audio"
	"github.com/Zyko0/EbitengineJam2026/core/entity"
	"github.com/Zyko0/EbitengineJam2026/input"
	"github.com/go-gl/mathgl/mgl64"
)

// CatchSequence is the scripted grab that plays when an enemy reaches the player
type CatchSequence struct {
	active  bool
	enemy   entity.Entity
	cfg     entity.CatchConfig
	elapsed float64
}

const catchLookLerp = 0.22 // camera turn rate toward the forced stare

func (s *CatchSequence) Active() bool {
	return s.active
}

// Start seizes control and freezes the grabbing enemy onto its strike.
func (s *CatchSequence) Start(e entity.Entity) {
	s.active = true
	s.enemy = e
	s.cfg = e.CatchConfig()
	s.elapsed = 0
	e.SetCaught(true)
}

// Update advances one tick. It returns true on the tick the blow lands, which is
// also when the grab releases: the player has been hit and the normal loop takes
// over again next tick.
func (s *CatchSequence) Update(p *Player, cam *Camera) (landed bool) {
	s.elapsed += tickSeconds

	// Drain mouse deltas every tick (discarded) so they don't pile up and snap the
	// view the moment free look returns.
	input.ProcessMouseMovement()

	// Force-look: yaw onto the foe, pitch to its configured up/down angle.
	ep := s.enemy.Position()
	to := mgl64.Vec3{ep.X() - p.Pos.X(), 0, ep.Z() - p.Pos.Z()}
	yaw, pitch := cam.YawPitch()
	if to.Len() > 1e-6 {
		yaw = approachAngle(yaw, math.Atan2(to.X(), to.Z()), catchLookLerp)
	}
	pitch += (s.cfg.LookPitch - pitch) * catchLookLerp
	cam.SetYawPitch(yaw, pitch)

	// Held in place: no movement, only momentum/bob decay stays coherent.
	p.Moving = false
	p.Running = false
	p.Update()

	if s.elapsed >= s.cfg.Duration {
		s.land(p)
		return true
	}

	return false
}

// land resolves the blow, releasing both the enemy and the player.
func (s *CatchSequence) land(p *Player) {
	// Shove the player horizontally away from the foe, falling back to a fixed
	// heading when standing exactly on top of it.
	ep := s.enemy.Position()
	away := mgl64.Vec3{p.Pos.X() - ep.X(), 0, p.Pos.Z() - ep.Z()}
	if away.Len() < 1e-6 {
		away = mgl64.Vec3{1, 0, 0}
	}
	p.TakeHit(away.Normalize().Mul(s.cfg.Knockback))
	// Play hit sound effect
	audio.PlayHit(p.Pos)

	s.enemy.SetCaught(false)
	s.enemy = nil
	s.active = false
}
