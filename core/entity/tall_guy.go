package entity

import (
	"math"

	xaudio "github.com/Zyko0/EbitengineJam2026/audio"
	"github.com/Zyko0/EbitengineJam2026/core/entity/poses"
	"github.com/Zyko0/EbitengineJam2026/logic"
	"github.com/go-gl/mathgl/mgl64"
)

// World is the slice of the level the tall guy needs: solid-cell lookups for
// line-of-sight and spawn-sightline raycasts, plus a grid path to route around
// walls when the straight line to the player is blocked. *level.Level satisfies it.
type World interface {
	Solid(x, z int) bool
	DiagBFSPath(sx, sz, tx, tz int) [][2]int
}

const (
	// TallGuyMinSpawnDist is the minimum clear sightline required to spawn
	TallGuyMinSpawnDist = 20.0

	tallGuyMaxScan       = 64.0 // how far the spawn ray probes for the end wall
	tallGuyTorsoY        = 1.6  // aim point for the line-of-sight / FOV test
	tallGuyVanishHideSec = 7.5  // consecutive seconds hidden before it vanishes
	tallGuyCooldownSec   = 10.0 // delay between a vanish and the next spawn
	tallGuyInitialDelay  = 4.0  // grace before the first spawn on a fresh level

	tallGuySpeed    = 0.045 // forward step per tick toward the player (blocks)
	tallGuyStopDist = 1.5   // never crowds closer than this to the player
	tallGuyProbe    = 0.4   // wall look-ahead so it stops short of a block, not in it

	// Spawn FOV half-cone (cosine of the angle off the view axis): tight so the
	// whole body shows comfortably centred when the guy first appears.
	tallGuySpawnCos = 0.96 // ~16 deg: spawns well inside the frame

	// Blink cadence: an even one-second-on, one-second-off flicker.
	tallGuyBlinkInterval = 1.0
)

// TallGuy is the slenderman-like silhouette
type TallGuy struct {
	*baseEntity
	hideTime float64 // consecutive seconds its line of sight to the player is broken
	vanished bool

	// Detour route used only while a wall blocks the straight line to the player:
	// a grid path to pathGoal (the player's cell), walked waypoint by waypoint.
	path     [][2]int
	pathIdx  int
	pathGoal [2]int
}

// tallGuyAnims is the spawn rotation: each new tall guy takes the next clip,
// starting with zombie run, then cycling back to the original drunk walk.
var tallGuyAnims = []poses.Animation{
	poses.ZombieRunAnim,
	poses.DrunkWalkAnim,
}

// NewTallGuy builds a tall guy standing (foot on the floor) at pos, playing anim.
func NewTallGuy(pos mgl64.Vec3, anim poses.Animation) *TallGuy {
	t := &TallGuy{
		baseEntity: newBaseEntity(mgl64.Vec3{pos.X(), 0, pos.Z()}, TallOne),
	}
	t.SetAnim(anim)

	return t
}

func (t *TallGuy) Update(ctx *Context) {
	const dt = 1.0 / logic.TPS

	// While grabbing the player it stops closing in and hit
	if t.Caught() {
		t.speed = 1
		t.baseEntity.update(ctx)
		return
	}

	// If the tall guy can't see the player, it contributes to the time to vanish
	if ctx.Disconnected || segmentBlocked(ctx.World, t.pos, ctx.PlayerPos) {
		t.hideTime += dt
		if t.hideTime >= tallGuyVanishHideSec {
			t.vanished = true
		}
	} else {
		t.hideTime = 0
	}

	if ctx.Disconnected {
		// Can't see, stops moving if player disconnected
		t.speed = 0
		t.baseEntity.update(ctx)
		return
	}

	// Always closing in: walk toward the player, sliding around any wall blocks.
	t.speed = 1
	t.stepToward(ctx.World, ctx.PlayerPos)

	t.baseEntity.update(ctx)
}

// Vanished reports that the guy has hidden long enough to disappear.
func (t *TallGuy) Vanished() bool {
	return t.vanished
}

// stepToward advances one tick toward the player
func (t *TallGuy) stepToward(w World, target mgl64.Vec3) {
	to := mgl64.Vec3{target.X() - t.pos.X(), 0, target.Z() - t.pos.Z()}
	dist := to.Len()
	if dist <= tallGuyStopDist {
		return
	}

	// Open ground: walk straight at the player and drop any stale detour.
	if !segmentBlocked(w, t.pos, target) {
		t.path = nil
		t.slideStep(w, to.Mul(1/dist))
		return
	}

	// A wall blocks the direct line: route around it. Recompute the grid path
	// whenever the player crosses into a new cell so it keeps tracking them.
	gx, gz := int(math.Floor(target.X())), int(math.Floor(target.Z()))
	if t.path == nil || gx != t.pathGoal[0] || gz != t.pathGoal[1] {
		cx, cz := int(math.Floor(t.pos.X())), int(math.Floor(t.pos.Z()))
		t.path = w.DiagBFSPath(cx, cz, gx, gz)
		t.pathGoal = [2]int{gx, gz}
		t.pathIdx = 1 // path[0] is the cell it already stands in
	}

	// Steer toward the next not-yet-reached waypoint, advancing as each is met.
	for t.pathIdx < len(t.path) {
		wp := mgl64.Vec3{float64(t.path[t.pathIdx][0]) + 0.5, 0, float64(t.path[t.pathIdx][1]) + 0.5}
		d := mgl64.Vec3{wp.X() - t.pos.X(), 0, wp.Z() - t.pos.Z()}
		if l := d.Len(); l > tallGuySpeed {
			t.slideStep(w, d.Mul(1/l))
			return
		}
		t.pos = mgl64.Vec3{wp.X(), t.pos.Y(), wp.Z()}
		t.pathIdx++
	}
}

// slideStep moves one tick along dir, sidestepping along whichever axis stays
// clear if dir itself is momentarily blocked (e.g. clipping a wall corner).
func (t *TallGuy) slideStep(w World, dir mgl64.Vec3) {
	if t.tryStep(w, dir) {
		return
	}
	ax := mgl64.Vec3{math.Copysign(1, dir.X()), 0, 0}
	az := mgl64.Vec3{0, 0, math.Copysign(1, dir.Z())}
	if math.Abs(dir.X()) >= math.Abs(dir.Z()) {
		if t.tryStep(w, az) {
			return
		}
		t.tryStep(w, ax)
	} else {
		if t.tryStep(w, ax) {
			return
		}
		t.tryStep(w, az)
	}
}

// tryStep moves one tick along dir if the cell just ahead is open.
func (t *TallGuy) tryStep(w World, dir mgl64.Vec3) bool {
	probe := t.pos.Add(dir.Mul(tallGuySpeed + tallGuyProbe))
	if w.Solid(int(math.Floor(probe.X())), int(math.Floor(probe.Z()))) {
		return false
	}
	t.pos = t.pos.Add(dir.Mul(tallGuySpeed))
	return true
}

// sees reports whether the player both faces pos (within cosThresh of the view
// axis) and has an unobstructed line to it.
func sees(ctx *Context, pos mgl64.Vec3, cosThresh float64) bool {
	eye := ctx.PlayerPos
	to := mgl64.Vec3{pos.X(), tallGuyTorsoY, pos.Z()}.Sub(eye)
	if to.Len() < 1e-6 {
		return true
	}
	if to.Normalize().Dot(ctx.ViewDir.Normalize()) < cosThresh {
		return false
	}
	return !segmentBlocked(ctx.World, eye, pos)
}

// segmentBlocked marches the horizontal segment a->b and reports a wall between
// the endpoints (both ends excluded, so the player's and target's own cells
// never count).
func segmentBlocked(w World, a, b mgl64.Vec3) bool {
	dx, dz := b.X()-a.X(), b.Z()-a.Z()
	dist := math.Hypot(dx, dz)
	if dist < 1e-6 {
		return false
	}
	n := int(dist/0.2) + 1
	for i := 1; i < n; i++ {
		t := float64(i) / float64(n)
		x := a.X() + dx*t
		z := a.Z() + dz*t
		if w.Solid(int(math.Floor(x)), int(math.Floor(z))) {
			return true
		}
	}
	return false
}

// TallGuyDirector owns the single live tall guy and handles its logic
type TallGuyDirector struct {
	MinDist float64 // clear sightline required to spawn (blocks)

	guy      *TallGuy
	cooldown float64 // seconds until a spawn is allowed again
	animIdx  int     // next slot in tallGuyAnims, advanced per spawn

	blinkOff   bool    // currently flickered out (skip rendering)
	blinkTimer float64 // seconds until the next blink toggle
}

func NewTallGuyDirector() *TallGuyDirector {
	return &TallGuyDirector{
		MinDist:  TallGuyMinSpawnDist,
		cooldown: tallGuyInitialDelay,
	}
}

func (d *TallGuyDirector) Update(ctx *Context) {
	const dt = 1.0 / logic.TPS

	if d.cooldown > 0 {
		d.cooldown -= dt
	}

	if d.guy == nil {
		// Never materialise during a blackout: the player can't see the spawn, and it
		// would only burn down its own vanish timer in the dark.
		if d.cooldown <= 0 && !ctx.Disconnected {
			if pos, ok := d.findSpawn(ctx); ok {
				d.guy = NewTallGuy(pos, tallGuyAnims[d.animIdx%len(tallGuyAnims)])
				d.animIdx++
				d.blinkOff = false
				d.blinkTimer = tallGuyBlinkInterval
				xaudio.PlayTallguyLaugh([3]float64(pos))
			}
		}
		return
	}

	d.guy.Update(ctx)
	if d.guy.Vanished() {
		d.guy = nil
		d.cooldown = tallGuyCooldownSec
		return
	}

	// While grabbing the player it stays solid: no blink flicker for the whole
	// strike, so the silhouette never vanishes mid-animation.
	if d.guy.Caught() {
		d.blinkOff = false
		d.blinkTimer = tallGuyBlinkInterval
		return
	}

	d.updateBlink(dt)
}

// Guy returns the live tall guy (regardless of the blink flicker) for catch
// detection, or nil when none is spawned.
func (d *TallGuyDirector) Guy() (Entity, bool) {
	return d.guy, d.guy != nil
}

// Renderable returns the live guy and whether it should be drawn this frame
func (d *TallGuyDirector) Renderable() (Entity, bool) {
	return d.guy, d.guy != nil && !d.blinkOff
}

func (d *TallGuyDirector) updateBlink(dt float64) {
	d.blinkTimer -= dt
	if d.blinkTimer > 0 {
		return
	}
	d.blinkTimer += tallGuyBlinkInterval
	d.blinkOff = !d.blinkOff
	xaudio.PlayClockTick([3]float64(d.guy.Position())) // tick each flicker, like a ticking clock
}

// findSpawn raycasts down the player's flattened view to the furthest visible
// voxel and returns that cell's centre, provided it is at least MinDist away and
// fully in frame. Walls along the way guarantee it never spawns occluded.
func (d *TallGuyDirector) findSpawn(ctx *Context) (mgl64.Vec3, bool) {
	dir := mgl64.Vec3{ctx.ViewDir.X(), 0, ctx.ViewDir.Z()}
	if dir.Len() < 1e-6 {
		return mgl64.Vec3{}, false
	}
	dir = dir.Normalize()
	origin := mgl64.Vec3{ctx.PlayerPos.X(), 0, ctx.PlayerPos.Z()}

	const step = 0.2
	var last mgl64.Vec3
	found := false
	for t := step; t <= tallGuyMaxScan; t += step {
		p := origin.Add(dir.Mul(t))
		if ctx.World.Solid(int(math.Floor(p.X())), int(math.Floor(p.Z()))) {
			break
		}
		last, found = p, true
	}
	if !found {
		return mgl64.Vec3{}, false
	}

	pos := mgl64.Vec3{math.Floor(last.X()) + 0.5, 0, math.Floor(last.Z()) + 0.5}
	if pos.Sub(origin).Len() < d.MinDist {
		return mgl64.Vec3{}, false
	}
	if !sees(ctx, pos, tallGuySpawnCos) {
		return mgl64.Vec3{}, false
	}

	return pos, true
}
