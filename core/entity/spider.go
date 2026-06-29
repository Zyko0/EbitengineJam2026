package entity

import (
	"math"
	"math/rand"

	xaudio "github.com/Zyko0/EbitengineJam2026/audio"
	"github.com/Zyko0/EbitengineJam2026/core/entity/poses"
	"github.com/Zyko0/EbitengineJam2026/graphics"
	"github.com/Zyko0/EbitengineJam2026/logic"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/hajimehoshi/ebiten/v2"
)

const (
	// spiderCeilingY hangs the spider just under the real ceiling (MapHeight 40),
	// so it patrols high overhead rather than floating mid-room.
	spiderCeilingY = 38.0
	spiderFloorY   = 0.0 // landed: feet on the floor, level with other foes

	spiderWanderSpeed = 0.1 // lazy ceiling drift between random rooms
	spiderHuntSpeed   = 0.2 // charge across the ceiling toward the player's room
	spiderDropSpeed   = 0.4 // straight plunge down the room's middle
	spiderClimbSpeed  = 0.3 // climb back up to the ceiling after a strike
	spiderGroundSpeed = 0.2 // crawl across the floor toward the player

	spiderArriveTol = 0.6 // XZ distance that counts as "reached" a cell centre
	spiderProbe     = 0.4 // wall look-ahead so it stops short of a block
	spiderCatchY    = 3.0 // only grabs once dropped to roughly head height

	// spiderFlipHeight is the climb height the ceiling-flip waits for, so the snap
	// back to upside-down happens up in the air, not poking through the floor.
	spiderFlipHeight = 5.0

	spiderWanderHold   = 2.5 // seconds to linger before picking a new wander room
	spiderGroundChase  = 6.0 // seconds it hunts on foot before giving up and climbing
	spiderHitCooldown  = 5.0 // seconds it leaves the player alone after landing a hit
	spiderMissCooldown = 3.0 // shorter breather if the floor chase never connected

	// Skitter loudness, lerped floor->ceiling by height. The ceiling sits ~38
	// units up, so distance attenuation alone leaves an overhead spider near
	// silent; lifting the base volume the higher it is makes the player hear it
	// roaming the room and approaching before it ever drops. Tune that cue here.
	spiderSkitterVol        = 0.6 // on the floor: let the spatial mix do the work
	spiderSkitterCeilingVol = 1.6 // up high: counter the distance and stay audible

	// The level is always a 4x4 grid of 16-cell rooms (see level.newGen): hard-code
	// it so the spider can find a room's middle without a level dependency.
	spiderRoomSize = 16
	spiderRoomGrid = 4
	spiderMapCells = spiderRoomSize * spiderRoomGrid
)

type spiderState int

const (
	spiderWander  spiderState = iota // drift the ceiling between random rooms
	spiderHunt                       // race along the ceiling to the player's room
	spiderDescend                    // straight vertical plunge down the room middle
	spiderGround                     // crawl the floor toward the player and hit
	spiderAscend                     // climb back up to resume the patrol
)

// SpiderStalker is the lone ceiling stalker (wearing the Spider profile). It
// drifts the ceiling upside-down until the player breaks into a run, then races
// to their room, flips legs-down and plunges through the middle, crawls the
// floor to hit the player, and climbs back into the dark for a breather. Exactly
// one exists per level.
type SpiderStalker struct {
	*baseEntity
	state spiderState
	rng   *rand.Rand

	// Ceiling/floor navigation, mirroring the tall guy's grid follower.
	path     [][2]int
	pathIdx  int
	pathGoal [2]int

	wanderTarget [2]int  // current random ceiling destination ({-1,-1} = none)
	column       [2]int  // locked descent cell (room middle) while dropping
	hold         float64 // wander-linger / ground-chase countdown
	cooldown     float64 // seconds before it may hunt again (set after an attack)
	wasCaught    bool    // grab edge: a landed hit ends the floor chase

	skitter *xaudio.SpatialPlayer // looping leg-scuttle, paused while it holds still
}

// NewSpiderStalker spawns the spider clinging to the ceiling above spawn (a
// ground-level open cell); it lifts itself to the ceiling height.
func NewSpiderStalker(spawn mgl64.Vec3) *SpiderStalker {
	s := &SpiderStalker{
		baseEntity:   newBaseEntity(mgl64.Vec3{spawn.X(), spiderCeilingY, spawn.Z()}, Spider),
		state:        spiderWander,
		rng:          rand.New(rand.NewSource(rand.Int63())),
		pathGoal:     [2]int{-1, -1},
		wanderTarget: [2]int{-1, -1},
		skitter:      xaudio.SpiderSkitter(),
	}
	s.SetAnim(poses.LowCrawlAnim)
	// Place the scuttle at the spider before starting it, otherwise it sits on
	// the listener at the origin and blares until the first Update positions it.
	s.skitter.SetPosition([3]float64(s.pos))
	s.skitter.Play()

	return s
}

func (s *SpiderStalker) Update(ctx *Context) {
	const dt = 1.0 / logic.TPS

	// Scuttle audio only while it actually moves: track this tick's displacement
	// and pan/gate the loop at the end, after every branch has settled s.pos.
	prev := s.pos
	defer s.updateSkitter(prev)

	// A landed grab releases the spider; if it was on the floor hunting, that hit
	// ends the chase, starts the climb back, and buys the player a long breather.
	if s.wasCaught && !s.Caught() && s.state == spiderGround {
		s.state = spiderAscend
		s.path = nil
		s.cooldown = spiderHitCooldown
	}
	s.wasCaught = s.Caught()
	if s.cooldown > 0 {
		s.cooldown -= dt
	}

	// Mid-grab: hold position and let the (plain crawl) animation play out.
	if s.Caught() {
		s.baseEntity.update(ctx)
		return
	}

	// Blind in the dark: it can neither see nor seize the player, so it freezes.
	if ctx.Disconnected {
		s.baseEntity.update(ctx)
		return
	}

	switch s.state {
	case spiderWander:
		s.wander(ctx)
		// Roused by a running player, but only once the post-attack breather ends.
		if ctx.PlayerRunning && s.cooldown <= 0 {
			s.state = spiderHunt
			s.path = nil
		}
	case spiderHunt:
		// If the player slows, drift off again.
		if !ctx.PlayerRunning {
			s.state = spiderWander
			s.path = nil
			break
		}
		// Re-target the player's room each tick so it tracks them between rooms,
		// then plunge once it sits above that room's middle.
		col := descendColumn(ctx.World, ctx.PlayerPos)
		if s.navStep(ctx.World, col, spiderHuntSpeed) {
			s.column = col
			s.state = spiderDescend
		}
	case spiderDescend:
		s.pos = mgl64.Vec3{
			float64(s.column[0]) + 0.5,
			s.pos.Y() - spiderDropSpeed,
			float64(s.column[1]) + 0.5,
		}
		if s.pos.Y() <= spiderFloorY {
			s.pos[1] = spiderFloorY
			s.hold = spiderGroundChase
			s.state = spiderGround
		}
	case spiderGround:
		// Crawl straight at the player on its own; the catch (armed at this height)
		// lands the hit. Give up and climb back after a while if it never connects.
		if s.hold -= dt; s.hold <= 0 {
			s.state = spiderAscend
			s.path = nil
			s.cooldown = spiderMissCooldown
			break
		}
		gx := clampInt(int(math.Floor(ctx.PlayerPos.X())), 0, spiderMapCells-1)
		gz := clampInt(int(math.Floor(ctx.PlayerPos.Z())), 0, spiderMapCells-1)
		s.navStep(ctx.World, [2]int{gx, gz}, spiderGroundSpeed)
	case spiderAscend:
		s.pos[1] += spiderClimbSpeed
		if s.pos.Y() >= spiderCeilingY {
			s.pos[1] = spiderCeilingY
			s.state = spiderWander
			s.path = nil
		}
	}

	s.baseEntity.update(ctx)
}

// updateSkitter pans the scuttle to the spider's new position and plays it only
// while it actually moved this tick, so a held-still, caught or frozen spider
// goes quiet. Distance attenuation in the spatial player does the rest: how
// dangerous it sounds is just how close it is.
func (s *SpiderStalker) updateSkitter(prev mgl64.Vec3) {
	// Counter the height attenuation: louder the higher it is, so the ceiling
	// patrol and approach are audible, not just the drop.
	t := s.pos.Y() / spiderCeilingY // 0 floor .. 1 ceiling
	s.skitter.SetBaseVolume(spiderSkitterVol + t*(spiderSkitterCeilingVol-spiderSkitterVol))
	s.skitter.SetPosition([3]float64(s.pos))
	if s.pos.Sub(prev).Len() > 1e-6 {
		if !s.skitter.IsPlaying() {
			s.skitter.Play()
		}
	} else if s.skitter.IsPlaying() {
		s.skitter.Pause()
	}
}

// wander drifts to a random room's middle, pausing on arrival before choosing
// the next, so the ceiling presence reads as restless rather than purposeful.
func (s *SpiderStalker) wander(ctx *Context) {
	if s.hold > 0 {
		s.hold -= 1.0 / logic.TPS
		return
	}
	if s.wanderTarget[0] < 0 {
		s.wanderTarget = s.randomCell(ctx.World)
		return
	}
	if s.navStep(ctx.World, s.wanderTarget, spiderWanderSpeed) {
		s.wanderTarget = [2]int{-1, -1}
		s.hold = spiderWanderHold
	}
}

// inverted reports whether the puppet hangs upside-down (legs up): true while it
// clings to or climbs back toward the ceiling, false once it has committed to the
// drop and through the floor crawl, where it stands legs-down. The flip back up
// waits until it is clear of the floor so the body never dips through the ground.
func (s *SpiderStalker) inverted() bool {
	switch s.state {
	case spiderDescend, spiderGround:
		return false
	case spiderAscend:
		return s.pos.Y() >= spiderFlipHeight
	default: // wander, hunt
		return true
	}
}

// AppendGeometry flips the camera-facing billboard vertically when the spider
// hangs from the ceiling, so it reads as upside-down without ever turning
// edge-on to the camera.
func (s *SpiderStalker) AppendGeometry(vx []ebiten.Vertex, ix []uint16, ctx *graphics.BillboardCtx) ([]ebiten.Vertex, []uint16) {
	if s.inverted() {
		ctx.Up = ctx.Up.Mul(-1)
	}

	return s.baseEntity.AppendGeometry(vx, ix, ctx)
}

// CatchConfig only arms the grab once the spider has dropped to head height: it
// is only ever this low through the plunge, the floor crawl and the start of the
// climb back, so the height check alone keeps it harmless up on the ceiling.
func (s *SpiderStalker) CatchConfig() CatchConfig {
	if s.pos.Y() <= spiderCatchY {
		return s.profile.Catch
	}

	return CatchConfig{}
}

// navStep advances one tick toward goal along a grid route, returning true once
// the spider sits within spiderArriveTol of the goal centre. Y is held constant,
// so this only ever moves it across the current plane. Mirrors TallGuy.stepToward.
func (s *SpiderStalker) navStep(w World, goal [2]int, speed float64) bool {
	gc := mgl64.Vec3{float64(goal[0]) + 0.5, s.pos.Y(), float64(goal[1]) + 0.5}
	to := mgl64.Vec3{gc.X() - s.pos.X(), 0, gc.Z() - s.pos.Z()}
	if to.Len() <= spiderArriveTol {
		return true
	}

	// Straight shot when nothing blocks the line; drop any stale detour.
	if !segmentBlocked(w, s.pos, gc) {
		s.path = nil
		s.slideStep(w, to.Normalize(), speed)
		return false
	}

	// A wall blocks the line: route around it, recomputing when the goal changes.
	if s.path == nil || goal != s.pathGoal {
		cx, cz := int(math.Floor(s.pos.X())), int(math.Floor(s.pos.Z()))
		s.path = w.DiagBFSPath(cx, cz, goal[0], goal[1])
		s.pathGoal = goal
		s.pathIdx = 1 // path[0] is the cell it already stands in
	}
	for s.pathIdx < len(s.path) {
		wp := mgl64.Vec3{float64(s.path[s.pathIdx][0]) + 0.5, 0, float64(s.path[s.pathIdx][1]) + 0.5}
		d := mgl64.Vec3{wp.X() - s.pos.X(), 0, wp.Z() - s.pos.Z()}
		if l := d.Len(); l > speed {
			s.slideStep(w, d.Mul(1/l), speed)
			return false
		}
		s.pos = mgl64.Vec3{wp.X(), s.pos.Y(), wp.Z()}
		s.pathIdx++
	}
	return false
}

// slideStep moves one tick along dir, sidestepping along whichever axis stays
// clear if dir itself is momentarily blocked (clipping a wall corner).
func (s *SpiderStalker) slideStep(w World, dir mgl64.Vec3, speed float64) {
	if s.tryStep(w, dir, speed) {
		return
	}
	ax := mgl64.Vec3{math.Copysign(1, dir.X()), 0, 0}
	az := mgl64.Vec3{0, 0, math.Copysign(1, dir.Z())}
	if math.Abs(dir.X()) >= math.Abs(dir.Z()) {
		if s.tryStep(w, az, speed) {
			return
		}
		s.tryStep(w, ax, speed)
	} else {
		if s.tryStep(w, ax, speed) {
			return
		}
		s.tryStep(w, az, speed)
	}
}

// tryStep moves one tick along dir if the cell just ahead is open.
func (s *SpiderStalker) tryStep(w World, dir mgl64.Vec3, speed float64) bool {
	probe := s.pos.Add(dir.Mul(speed + spiderProbe))
	if w.Solid(int(math.Floor(probe.X())), int(math.Floor(probe.Z()))) {
		return false
	}
	s.pos = s.pos.Add(dir.Mul(speed))

	return true
}

// randomCell picks the open middle of a random room as the next wander target.
func (s *SpiderStalker) randomCell(w World) [2]int {
	rx, rz := s.rng.Intn(spiderRoomGrid), s.rng.Intn(spiderRoomGrid)

	return nearestOpenCell(w, rx*spiderRoomSize+8, rz*spiderRoomSize+8)
}

// descendColumn returns the open cell nearest the middle of the room the player
// currently stands in: the spider's drop column.
func descendColumn(w World, player mgl64.Vec3) [2]int {
	rx := clampInt(int(math.Floor(player.X()))/spiderRoomSize, 0, spiderRoomGrid-1)
	rz := clampInt(int(math.Floor(player.Z()))/spiderRoomSize, 0, spiderRoomGrid-1)

	return nearestOpenCell(w, rx*spiderRoomSize+8, rz*spiderRoomSize+8)
}

// nearestOpenCell spirals out from (cx,cz) for the closest non-solid cell, so a
// room middle blocked by a pillar still yields a reachable drop point. Mirrors
// level.gen.nearestOpen.
func nearestOpenCell(w World, cx, cz int) [2]int {
	for r := 0; r < spiderRoomSize; r++ {
		for dz := -r; dz <= r; dz++ {
			for dx := -r; dx <= r; dx++ {
				if iabs(dx) != r && iabs(dz) != r {
					continue // ring perimeter only
				}
				x, z := cx+dx, cz+dz
				if x >= 0 && x < spiderMapCells && z >= 0 && z < spiderMapCells && !w.Solid(x, z) {
					return [2]int{x, z}
				}
			}
		}
	}

	return [2]int{cx, cz}
}

func iabs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
