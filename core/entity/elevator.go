package entity

import (
	"math"

	"github.com/Zyko0/EbitengineJam2026/graphics"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/hajimehoshi/ebiten/v2"
)

// Elevator is the level exit. It exists from level start holding its injected
// anchor, but stays inactive (hidden, untriggerable) until the button is pressed.
type Elevator struct {
	Pos     mgl64.Vec3
	Forward mgl64.Vec3 // away from the backing wall, into the room (axis-aligned)
	Active  bool       // has arrived: the button was pressed
	RiseY   float64    // vertical platform offset, driven by the exit sequence

	arrivedAudio bool
}

const (
	elevHalf = 1.0 // platform footprint half-size (2x2)

	elevPlatformHalfH = 0.06 // platform slab half-thickness

	// Wall panel, mounted on the backing wall (the wall along -Forward).
	elevCadranHW = 0.9 // black panel half-width
	elevCadranHH = 1.5 // black panel half-height (spans ~0..3 tall)
	elevCadranCY = 1.5 // black panel center height
	elevCadranHT = 0.04

	elevPlateHW = 0.9  // white sign half-width
	elevPlateHH = 0.32 // white sign half-height
	elevPlateCY = 3.35 // white sign center height (just above the cadran)
	elevPlateHT = 0.04

	// EXIT glyphs, laid out across the white plate.
	elevGlyphHW    = 0.13 // glyph half-width
	elevGlyphHH    = 0.20 // glyph half-height
	elevGlyphPitch = 0.40 // spacing between glyph centers
	elevStrokeW    = 0.03 // glyph stroke half-thickness (world units)
)

var (
	elevPlatformColor = [3]float64{0.05, 0.05, 0.05}
	elevCadranColor   = [3]float64{0.06, 0.06, 0.07}
	elevPlateColor    = [3]float64{0.92, 0.92, 0.92}
	elevTextColor     = [3]float64{0.04, 0.04, 0.04}
)

// elevGlyphs are block letters as centerline strokes in a normalized [-1,1] box
// (x = right, y = up); each stroke gets thickness elevStrokeW when drawn.
var elevGlyphs = map[rune][][4]float64{
	'E': {{-0.7, -1, -0.7, 1}, {-0.7, 1, 0.7, 1}, {-0.7, 0, 0.4, 0}, {-0.7, -1, 0.7, -1}},
	'X': {{-0.7, -1, 0.7, 1}, {-0.7, 1, 0.7, -1}},
	'I': {{0, -1, 0, 1}, {-0.5, 1, 0.5, 1}, {-0.5, -1, 0.5, -1}},
	'T': {{-0.7, 1, 0.7, 1}, {0, 1, 0, -1}},
}

func NewElevator(pos, forward mgl64.Vec3) *Elevator {
	return &Elevator{
		Pos:     pos,
		Forward: forward.Normalize(),
	}
}

// right is the cage's local lateral axis (panel span direction).
func (e *Elevator) right() mgl64.Vec3 {
	return mgl64.Vec3{0, 1, 0}.Cross(e.Forward).Normalize()
}

// Contains reports whether the player stands within the platform footprint.
func (e *Elevator) Contains(playerPos mgl64.Vec3) bool {
	rel := playerPos.Sub(e.Pos)
	rx := rel.Dot(e.right())
	fz := rel.Dot(e.Forward)

	return math.Abs(rx) < elevHalf && math.Abs(fz) < elevHalf
}

// AppendGeometry emits the rising platform plus the fixed wall panel (cadran,
// white plate and EXIT lettering). All parts are solid (signDist -1) so the
// entity shader's WallBias keeps the backing wall from bleeding through.
func (e *Elevator) AppendGeometry(vx []ebiten.Vertex, ix []uint16, c *graphics.MeshCtx) ([]ebiten.Vertex, []uint16) {
	right := e.right()
	up := mgl64.Vec3{0, 1, 0}

	// Rising platform slab.
	platCenter := e.Pos.Add(mgl64.Vec3{0, e.RiseY + elevPlatformHalfH, 0})
	vx, ix = graphics.AppendBox3D(vx, ix, c, platCenter,
		axisAlignedHalf(right, e.Forward, elevHalf, elevPlatformHalfH, elevHalf),
		elevPlatformColor[0], elevPlatformColor[1], elevPlatformColor[2], -1)

	// Wall boundary the panel mounts on (platform rear face, y=0 at the floor).
	wall := e.Pos.Sub(e.Forward.Mul(elevHalf))

	// panelBox mounts an axis-aligned box on the wall: localR shifts along the
	// wall, y is its center height, proud pushes its front out along Forward.
	panelBox := func(localR, y, hw, hh, ht, proud float64, col [3]float64) {
		center := wall.
			Add(right.Mul(localR)).
			Add(mgl64.Vec3{0, y + e.RiseY, 0}). // panel rides up with the platform
			Add(e.Forward.Mul(proud + ht))
		vx, ix = graphics.AppendBox3D(vx, ix, c, center,
			axisAlignedHalf(right, e.Forward, hw, hh, ht), col[0], col[1], col[2], -1)
	}

	panelBox(0, elevCadranCY, elevCadranHW, elevCadranHH, elevCadranHT, 0, elevCadranColor)
	plateProud := 2 * elevCadranHT
	panelBox(0, elevPlateCY, elevPlateHW, elevPlateHH, elevPlateHT, plateProud, elevPlateColor)

	// EXIT lettering, sitting proud of the plate front.
	plateCenter := wall.
		Add(mgl64.Vec3{0, elevPlateCY + e.RiseY, 0}).
		Add(e.Forward.Mul(plateProud + 2*elevPlateHT))
	stroke := func(x0, y0, x1, y1 float64) {
		p0 := mgl64.Vec2{x0, y0}
		d := mgl64.Vec2{x1 - x0, y1 - y0}
		if d.Len() < 1e-9 {
			return
		}
		du := d.Normalize()
		nu := mgl64.Vec2{-du.Y(), du.X()}.Mul(elevStrokeW)
		p1 := mgl64.Vec2{x1, y1}
		toWorld := func(p mgl64.Vec2) mgl64.Vec3 {
			// Negate the lateral axis so the word reads correctly from the room
			// side (the panel's right vector faces the other way).
			return plateCenter.Add(right.Mul(-p.X())).Add(up.Mul(p.Y()))
		}
		vx, ix = graphics.AppendQuad3D(vx, ix, c,
			toWorld(p0.Sub(nu)), toWorld(p1.Sub(nu)), toWorld(p1.Add(nu)), toWorld(p0.Add(nu)),
			elevTextColor[0], elevTextColor[1], elevTextColor[2], -1)
	}
	word := []rune{'E', 'X', 'I', 'T'}
	x0 := -elevGlyphPitch * float64(len(word)-1) / 2 // centered run of glyphs
	for i, r := range word {
		gx := x0 + float64(i)*elevGlyphPitch
		for _, s := range elevGlyphs[r] {
			stroke(
				gx+s[0]*elevGlyphHW, s[1]*elevGlyphHH,
				gx+s[2]*elevGlyphHW, s[3]*elevGlyphHH,
			)
		}
	}

	return vx, ix
}
