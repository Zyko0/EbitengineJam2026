package entity

import (
	"math"
	"math/rand"

	"github.com/Zyko0/EbitengineJam2026/core/entity/poses"
	"github.com/Zyko0/EbitengineJam2026/graphics"
	"github.com/Zyko0/EbitengineJam2026/logic"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/hajimehoshi/ebiten/v2"
)

const (
	SilhouetteGapStart      = 35.0
	SilhouetteKillGap       = 1.5
	SilhouetteBudgetSeconds = 20.0

	silhouetteClosePerTick = (SilhouetteGapStart - SilhouetteKillGap) / (SilhouetteBudgetSeconds * logic.TPS)
)

// WhiteSilhouette is the white figure that exists only while the player is
// disconnected
type WhiteSilhouette struct {
	*baseEntity
	gap     float64    // remaining distance to the player (blocks): the run-wide budget
	bearing mgl64.Vec3 // horizontal unit heading from the player, re-anchored each blackout
	disc    bool       // disconnected last tick, for rising-edge detection
	dead    bool
}

func NewWhiteSilhouette() *WhiteSilhouette {
	s := &WhiteSilhouette{
		baseEntity: newBaseEntity(mgl64.Vec3{}, Humanoid),
		gap:        SilhouetteGapStart,
		bearing:    mgl64.Vec3{0, 0, 1},
	}
	s.SetAnim(poses.JazzDancingAnim)

	return s
}

// Update advances the silhouette one tick. While disconnected it closes the gap
// (re-anchoring its bearing to a fresh random angle on each blackout); while
// connected it simply holds its budget, unseen. It always repositions relative to
// the player so the reveal fade catches it in the right spot.
func (s *WhiteSilhouette) Update(playerPos mgl64.Vec3, disconnected bool) {
	if disconnected {
		if !s.disc {
			s.bearing = randomHeading() // fresh blackout: anchor it at a new random angle
		}
		s.gap = max(s.gap-silhouetteClosePerTick, 0)
		if s.gap <= SilhouetteKillGap {
			s.dead = true
		}
	}
	s.disc = disconnected

	p := playerPos.Add(s.bearing.Mul(s.gap))
	s.pos = mgl64.Vec3{p.X(), 0, p.Z()}
	s.baseEntity.update(&Context{PlayerPos: playerPos})
}

// Dead reports that the silhouette has closed to the kill distance.
func (s *WhiteSilhouette) Dead() bool { return s.dead }

// Budget reports the remaining run-wide disconnect allowance in [0,1]: 1 while
// the silhouette sits at its starting distance, 0 at the kill range.
func (s *WhiteSilhouette) Budget() float64 {
	b := (s.gap - SilhouetteKillGap) / (SilhouetteGapStart - SilhouetteKillGap)
	return max(0, min(1, b))
}

// AppendGeometry builds the silhouette's billboard. Colour is left to the
// caller's dedicated white pass, which overwrites every vertex, since this puppet
// is drawn against the blackout rather than in the black entity batch.
func (s *WhiteSilhouette) AppendGeometry(vx []ebiten.Vertex, ix []uint16, camPos, right mgl64.Vec3, pv *mgl64.Mat4, sw, sh int) ([]ebiten.Vertex, []uint16) {
	ctx := &graphics.BillboardCtx{
		PosRel:  s.pos.Sub(camPos),
		Right:   right,
		Up:      mgl64.Vec3{0, 1, 0},
		TF:      pv,
		ScreenW: sw,
		ScreenH: sh,
	}
	return s.baseEntity.AppendGeometry(vx, ix, ctx)
}

// randomHeading returns a horizontal unit heading at a uniformly random angle, so
// each blackout drops the silhouette at a fresh bearing around the player.
func randomHeading() mgl64.Vec3 {
	sin, cos := math.Sincos(rand.Float64() * 2 * math.Pi)
	return mgl64.Vec3{sin, 0, cos}
}
