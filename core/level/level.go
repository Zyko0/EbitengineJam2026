package level

import (
	"math"

	"github.com/go-gl/mathgl/mgl64"
)

const MaxWallInts = 128

// Level packs walls row-major: data[z*intsPerRow + x/32], intsPerRow = ceil(W/32),
// matching scene.kage / entity.kage so Go-side wall hits mirror the render exactly.
type Level struct {
	data    []int32
	width   int
	depth   int
	walls         []bool
	circuit       []mgl64.Vec3 // Spinner lane loop (room centers), see patrol.go
	directCircuit []mgl64.Vec3 // PatrollerDirect loop, simplified for diagonal movement
}

func NewLevel(width, depth int, walls []bool) *Level {
	intsPerRow := (width + 31) / 32
	data := make([]int32, depth*intsPerRow)
	for z := 0; z < depth; z++ {
		for x := 0; x < width; x++ {
			if walls[x+z*width] {
				data[z*intsPerRow+x/32] |= 1 << (x % 32)
			}
		}
	}
	return &Level{data: data, width: width, depth: depth, walls: walls}
}

func (l *Level) Width() int { return l.width }
func (l *Level) Depth() int { return l.depth }

// Solid reports whether cell (x,z) is wall; out-of-bounds reads as wall.
func (l *Level) Solid(x, z int) bool {
	if x < 0 || x >= l.width || z < 0 || z >= l.depth {
		return true
	}
	return l.walls[x+z*l.width]
}

// PatrolCircuit returns the Spinner's closed lane loop as world waypoints
// (§8.2). Empty if the level has no loop.
func (l *Level) PatrolCircuit() []mgl64.Vec3 { return l.circuit }

// DirectCircuit returns the PatrollerDirect's closed loop with simplified
// line-of-sight waypoints for diagonal movement. Empty if unavailable.
func (l *Level) DirectCircuit() []mgl64.Vec3 { return l.directCircuit }

// Data returns wall data padded to MaxWallInts int32s for the shader uniform.
func (l *Level) Data() []int32 {
	out := make([]int32, MaxWallInts)
	copy(out, l.data)
	return out
}

func floorVec3(v mgl64.Vec3) mgl64.Vec3 {
	return mgl64.Vec3{math.Floor(v.X()), math.Floor(v.Y()), math.Floor(v.Z())}
}

// AppendAround collects the solid cells overlapping the box (pos, bounds) for
// the player's AABB collision sweep.
func (l *Level) AppendAround(neighbours []mgl64.Vec3, pos, bounds mgl64.Vec3) []mgl64.Vec3 {
	half := bounds.Mul(0.5)
	minp := floorVec3(pos.Sub(half))
	maxp := floorVec3(pos.Add(half))
	for x := minp.X(); x <= maxp.X(); x++ {
		xi := int(x)
		if xi < 0 || xi >= l.width {
			continue
		}
		for z := minp.Z(); z <= maxp.Z(); z++ {
			zi := int(z)
			if zi < 0 || zi >= l.depth {
				continue
			}
			if !l.walls[xi+zi*l.width] {
				continue
			}
			for y := minp.Y(); y <= maxp.Y(); y++ {
				neighbours = append(neighbours, mgl64.Vec3{x, y, z})
			}
		}
	}
	return neighbours
}
