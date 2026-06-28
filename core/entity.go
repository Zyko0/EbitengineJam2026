package core

import (
	"math"
	"sort"

	xaudio "github.com/Zyko0/EbitengineJam2026/audio"
	"github.com/Zyko0/EbitengineJam2026/core/entity"
	"github.com/Zyko0/EbitengineJam2026/core/level"
	"github.com/Zyko0/EbitengineJam2026/graphics"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/hajimehoshi/ebiten/v2"
)

const buttonAutoPressRange = 2. // player proximity that "presses" the button

type EntityManager struct {
	entities []entity.Entity
	button   *entity.Button
	elevator *entity.Elevator        // holds the exit anchor; inactive until pressed
	tallGuy  *entity.TallGuyDirector // spawns/recycles the lone tall guy
}

// NewEntityManager populates a level: button, patroller and all placed entities.
func NewEntityManager(m *level.Manifest) *EntityManager {
	mgr := &EntityManager{
		button:   entity.NewButton(m.ButtonPos, m.ButtonFacing),
		elevator: entity.NewElevator(m.ElevatorPos, m.ElevatorFwd),
		tallGuy:  entity.NewTallGuyDirector(),
	}

	// Exactly one ceiling spider per level.
	mgr.entities = append(mgr.entities, entity.NewSpiderStalker(m.SpiderSpawn))

	if len(m.Circuit) >= 2 {
		mgr.entities = append(mgr.entities, entity.NewPatrollerWall(m.Circuit[0], m.Circuit))
	}
	if len(m.DirectCircuit) >= 2 {
		mgr.entities = append(mgr.entities, entity.NewPatrollerDirect(m.DirectCircuit[0], m.DirectCircuit))
	}

	return mgr
}

// CatchCandidate returns the nearest enemy whose catch radius the player has
// entered, or nil if none is in reach. The manager only reports the grabber; the
// caller owns the grab sequence. A caught entity stays the candidate so an
// in-progress grab keeps resolving against the same foe.
func (m *EntityManager) CatchCandidate(playerPos mgl64.Vec3) entity.Entity {
	var best entity.Entity
	bestD := math.MaxFloat64
	consider := func(e entity.Entity) {
		c := e.CatchConfig()
		if !c.Catchable() {
			return
		}
		dx := e.Position().X() - playerPos.X()
		dz := e.Position().Z() - playerPos.Z()
		d := math.Hypot(dx, dz)
		if d <= c.Radius && d < bestD {
			best, bestD = e, d
		}
	}
	for _, e := range m.entities {
		consider(e)
	}
	if g, ok := m.tallGuy.Guy(); ok {
		consider(g)
	}

	return best
}

func (m *EntityManager) Update(ctx *entity.Context) {
	for _, e := range m.entities {
		e.Update(ctx)
	}
	m.tallGuy.Update(ctx)

	// Press the button by walking up to it; that activates the exit elevator
	// waiting at the injected anchor.
	if m.button != nil && !m.button.Pressed {
		d := ctx.PlayerPos.Sub(m.button.Pos)
		if d.X()*d.X()+d.Z()*d.Z() < buttonAutoPressRange*buttonAutoPressRange {
			xaudio.PlayButtonPush([3]float64(m.button.Pos))
			m.button.Pressed = true
			m.elevator.Active = true
		}
	}
}

func (m *EntityManager) AppendGeometry(vx []ebiten.Vertex, ix []uint16, camPos, right mgl64.Vec3, pv *mgl64.Mat4, sw, sh int) ([]ebiten.Vertex, []uint16) {
	worldUp := mgl64.Vec3{0, 1, 0}
	mctx := &graphics.MeshCtx{
		CamPos:  camPos,
		TF:      pv,
		ScreenW: sw,
		ScreenH: sh,
	}

	// The entity pass has no depth buffer, so submission order decides overlap;
	// append farthest first (painter's algorithm) so nearer things stay on top.
	type drawer struct {
		dist float64
		fn   func([]ebiten.Vertex, []uint16) ([]ebiten.Vertex, []uint16)
	}
	var drawers []drawer
	addPuppet := func(e entity.Entity) {
		posRel := e.Position().Sub(camPos)
		ctx := &graphics.BillboardCtx{PosRel: posRel, Right: right, Up: worldUp, TF: pv, ScreenW: sw, ScreenH: sh}
		drawers = append(drawers, drawer{posRel.Len(),
			func(vx []ebiten.Vertex, ix []uint16) ([]ebiten.Vertex, []uint16) {
				return e.AppendGeometry(vx, ix, ctx)
			}})
	}
	for _, e := range m.entities {
		addPuppet(e)
	}
	if g, ok := m.tallGuy.Renderable(); ok {
		addPuppet(g)
	}
	if m.button != nil {
		drawers = append(drawers, drawer{m.button.Pos.Sub(camPos).Len(),
			func(vx []ebiten.Vertex, ix []uint16) ([]ebiten.Vertex, []uint16) {
				return entity.AppendButton(vx, ix, m.button, mctx)
			}})
	}
	if m.elevator.Active {
		drawers = append(drawers, drawer{m.elevator.Pos.Sub(camPos).Len(),
			func(vx []ebiten.Vertex, ix []uint16) ([]ebiten.Vertex, []uint16) {
				return m.elevator.AppendGeometry(vx, ix, mctx)
			}})
	}
	sort.Slice(drawers, func(i, j int) bool {
		return drawers[i].dist > drawers[j].dist
	})
	for _, d := range drawers {
		vx, ix = d.fn(vx, ix)
	}

	return vx, ix
}
