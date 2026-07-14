package core

import (
	"math"

	xaudio "github.com/Zyko0/EbitengineJam2026/audio"
	"github.com/Zyko0/EbitengineJam2026/core/entity"
	"github.com/Zyko0/EbitengineJam2026/input"
	"github.com/go-gl/mathgl/mgl64"
)

// ExitSequence is the scripted ride that plays once the player steps in the elevator
type ExitSequence struct {
	active   bool
	phase    exitPhase
	elevator *entity.Elevator
}

type exitPhase int

const (
	exitWalk exitPhase = iota // auto-walk to the platform center
	exitTurn                  // rotate to face into the room
	exitRise                  // ride up, free look restored
)

const (
	exitWalkSpeed  = PlayerMovementSpeed
	exitArriveEps  = 0.05
	exitTurnLerp   = 0.16
	exitTurnEps    = 0.02
	exitRiseSpeed  = 0.03
	exitRiseTarget = 12.0
	exitFloorY     = 0.5 + PlayerHeight/2 // grounded player center height
)

func (s *ExitSequence) Active() bool {
	return s.active
}

// Done reports the ride has finished raising, the cue to regenerate the level.
func (s *ExitSequence) Done() bool {
	return s.active && s.phase == exitRise && s.elevator.RiseY >= exitRiseTarget
}

// Start hands control to the sequence. The caller force-reconnects first.
func (s *ExitSequence) Start(e *entity.Elevator) {
	s.active = true
	s.phase = exitWalk
	s.elevator = e
	xaudio.PlayElevatorLeave([3]float64(e.Pos))
}

// Update advances one tick of the sequence, driving the player and camera
// directly. The look delta is only applied once the ride begins.
func (s *ExitSequence) Update(p *Player, cam *Camera) {
	yawoff, pitchoff := input.Look()

	switch s.phase {
	case exitWalk:
		d := mgl64.Vec3{
			s.elevator.Pos.X() - p.Pos.X(),
			0,
			s.elevator.Pos.Z() - p.Pos.Z(),
		}
		dist := d.Len()
		if dist <= exitArriveEps {
			p.Pos = mgl64.Vec3{s.elevator.Pos.X(), p.Pos.Y(), s.elevator.Pos.Z()}
			p.Moving = false
			s.phase = exitTurn
			break
		}
		p.Pos = p.Pos.Add(d.Mul(math.Min(exitWalkSpeed, dist) / dist))
		p.Moving = true
		p.Running = false

	case exitTurn:
		p.Moving = false
		yaw, pitch := cam.YawPitch()
		targetYaw := math.Atan2(s.elevator.Forward.X(), s.elevator.Forward.Z())
		yaw = approachAngle(yaw, targetYaw, exitTurnLerp)
		pitch += (0 - pitch) * exitTurnLerp
		cam.SetYawPitch(yaw, pitch)
		if math.Abs(angleDiff(yaw, targetYaw)) < exitTurnEps && math.Abs(pitch) < exitTurnEps {
			cam.SetYawPitch(targetYaw, 0)
			s.phase = exitRise
		}

	case exitRise:
		p.Moving = false
		yaw, pitch := cam.YawPitch()
		yaw += yawoff
		pitch = math.Max(math.Min(pitch+pitchoff, math.Pi/2), -math.Pi/2)
		cam.SetYawPitch(yaw, pitch)

		s.elevator.RiseY = math.Min(exitRiseTarget, s.elevator.RiseY+exitRiseSpeed)
		p.Pos = mgl64.Vec3{p.Pos.X(), exitFloorY + s.elevator.RiseY, p.Pos.Z()}
	}

	p.Update() // keep momentum / head bob / footstep audio coherent
}

// angleDiff returns the shortest signed difference a-b wrapped to [-pi, pi].
func angleDiff(a, b float64) float64 {
	d := math.Mod(a-b+math.Pi, 2*math.Pi)
	if d < 0 {
		d += 2 * math.Pi
	}
	return d - math.Pi
}

// approachAngle lerps a toward b by t along the shortest arc.
func approachAngle(a, b, t float64) float64 {
	return a - angleDiff(a, b)*t
}
