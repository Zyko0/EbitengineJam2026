package core

import (
	"math"

	xaudio "github.com/Zyko0/EbitengineJam2026/audio"
	"github.com/Zyko0/EbitengineJam2026/logic"
	"github.com/go-gl/mathgl/mgl64"
)

const (
	PlayerMovementSpeed       = 0.075
	PlayerRunSpeedMultiplier  = 1.5
	PlayerDarkSpeedMultiplier = 1. //0.5 // disconnected: feeling your way
	PlayerWidth               = 1.
	PlayerHeight              = 3.

	PlayerMaxHP = 5

	// After a hit the player gets brief invulnerability frame
	PlayerInvulnSeconds = 1.5
	PlayerInvulnTicks   = int(PlayerInvulnSeconds * logic.TPS)
	knockbackDecay      = 0.85  // per-tick velocity multiplier
	knockbackStopEps    = 0.004 // below this the shove is dropped
)

// Head bob

const (
	bobWalkFreq = 0.19
	bobRunFreq  = 0.32
	bobWalkAmpY = 0.05
	bobRunAmpY  = 0.09
	bobAmpXFrac = 0.30 // Lateral is 30% of vertical
	bobDecay    = 0.82 // Per-tick decay factor when not stepping
)

type playerBob struct {
	phase float64
	Y     float64
	X     float64
}

func (b *playerBob) update(moving, running bool) {
	if moving {
		freq := bobWalkFreq
		ampY := bobWalkAmpY
		if running {
			freq = bobRunFreq
			ampY = bobRunAmpY
		}
		b.phase += freq
		b.Y = math.Sin(b.phase) * ampY
		b.X = math.Sin(b.phase*0.5) * ampY * bobAmpXFrac
	} else {
		b.Y *= bobDecay
		b.X *= bobDecay
	}
}

// Momentum

const (
	playerAccel    = 0.18 // fraction per tick toward target speed
	playerDecel    = 0.30 // fraction per tick toward 0
	FovSprintBonus = 4.0  // degrees added to FOV while sprinting
	fovLerpFactor  = 0.22 // fraction per tick toward target FOV
)

type playerMomentum struct {
	Speed float64
}

func (m *playerMomentum) update(moving, running, dark bool) {
	target := 0.0
	if moving {
		target = PlayerMovementSpeed
		if dark {
			target *= PlayerDarkSpeedMultiplier
		}
		if running {
			target *= PlayerRunSpeedMultiplier
		}
	}
	factor := playerDecel
	if target > m.Speed {
		factor = playerAccel
	}
	m.Speed += (target - m.Speed) * factor
}

// Player

type Player struct {
	Pos       mgl64.Vec3
	Fov       float64
	HP        int
	Moving    bool
	Running   bool
	Dark      bool
	Knockback mgl64.Vec3 // residual horizontal shove from a hit, decays each tick
	Invuln    int        // remaining invulnerability ticks (no new grab)
	Bob       playerBob
	Momentum  playerMomentum
	footstep  *xaudio.Footstep
}

func newPlayer(pos mgl64.Vec3, fov float64) *Player {
	footstep, src := xaudio.NewFootstepWithReverb(0.5)
	player, _ := xaudio.NewPlayer(src)
	player.Play()

	return &Player{
		Pos:      pos,
		Fov:      fov,
		HP:       PlayerMaxHP,
		footstep: footstep,
	}
}

func (p *Player) Update() {
	// Momentum
	p.Momentum.update(p.Moving, p.Running, p.Dark)

	// FOV
	targetFov := float64(logic.CameraFoV)
	if p.Moving && p.Running {
		targetFov += FovSprintBonus
	}
	p.Fov += (targetFov - p.Fov) * fovLerpFactor

	// Head bob
	p.Bob.update(p.Moving, p.Running)

	// Footstep audio
	walkSpeed := PlayerMovementSpeed
	if p.Dark {
		walkSpeed *= PlayerDarkSpeedMultiplier
	}
	runSpeed := walkSpeed * PlayerRunSpeedMultiplier
	blend := (p.Momentum.Speed - walkSpeed) / (runSpeed - walkSpeed)
	p.footstep.Active = p.Moving
	p.footstep.Blend = max(0, min(1, blend))
	p.footstep.IntervalScale = 1
	if p.Dark {
		p.footstep.IntervalScale = 1 / PlayerDarkSpeedMultiplier
	}
}

// TakeHit applies one grab's worth of damage: drop an HP, start the i-frames and
// arm the knockback the normal physics then carries (and collides) outward.
func (p *Player) TakeHit(knockback mgl64.Vec3) {
	p.HP--
	p.Invuln = PlayerInvulnTicks
	p.Knockback = knockback
}

// stepKnockback returns the horizontal shove to fold into this tick's movement
// and decays the residual, dropping it once it falls below the cutoff.
func (p *Player) stepKnockback() mgl64.Vec3 {
	k := p.Knockback
	p.Knockback = p.Knockback.Mul(knockbackDecay)
	if p.Knockback.Len() < knockbackStopEps {
		p.Knockback = mgl64.Vec3{}
	}

	return k
}
