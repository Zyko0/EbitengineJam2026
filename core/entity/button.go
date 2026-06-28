package entity

import (
	"github.com/Zyko0/EbitengineJam2026/graphics"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/hajimehoshi/ebiten/v2"
)

// Button is a wall-mounted control panel: a light-gray mural plate dotted with
// small colored studs, plus a round pressable button in the middle. It is
// axis-aligned (mounted on a vertical wall), so Facing is a horizontal unit
// normal pointing into the room. Pos is the plate's geometric center.
type Button struct {
	Pos     mgl64.Vec3
	Facing  mgl64.Vec3 // outward normal, into the room (axis-aligned, horizontal)
	Pressed bool
}

const (
	buttonCenterY = 2.0  // ~2/3 of the 3-tall player, sits just below the head
	buttonPlateHW = 0.55 // plate half-width
	buttonPlateHH = 0.55 // plate half-height
	buttonPlateHT = 0.06 // plate half-thickness
	buttonRadius  = 0.22
	buttonRaise   = 0.12
	buttonSegs    = 20

	buttonStudHalf  = 0.04 // small studs on the plate corners
	buttonStudInset = 0.18 // distance of studs in from the plate edges
)

// buttonAccent is the green shown on the round button once it is pressed.
var buttonAccent = [3]float64{0.15, 0.78, 0.22}

// buttonStudColor is the light-gray of the corner screws: darker than the
// button, lighter than the plate.
var buttonStudColor = [3]float64{0.85, 0.85, 0.85}

// buttonStuds are the (right, up) plate-local positions of the corner studs.
var buttonStuds = [][2]float64{
	{-buttonPlateHW + buttonStudInset, buttonPlateHH - buttonStudInset},
	{buttonPlateHW - buttonStudInset, buttonPlateHH - buttonStudInset},
	{-buttonPlateHW + buttonStudInset, -buttonPlateHH + buttonStudInset},
	{buttonPlateHW - buttonStudInset, -buttonPlateHH + buttonStudInset},
}

func NewButton(pos, facing mgl64.Vec3) *Button {
	return &Button{Pos: pos, Facing: facing.Normalize()}
}

// AppendButton emits the panel geometry: backing plate, colored studs and the
// raised round button (green once pressed).
func AppendButton(vx []ebiten.Vertex, ix []uint16, b *Button, c *graphics.MeshCtx) ([]ebiten.Vertex, []uint16) {
	up := mgl64.Vec3{0, 1, 0}
	right := up.Cross(b.Facing).Normalize()

	// Backing plate (slim box flush against the wall).
	vx, ix = graphics.AppendBox3D(vx, ix, c, b.Pos,
		axisAlignedHalf(right, b.Facing, buttonPlateHW, buttonPlateHH, buttonPlateHT),
		0.72, 0.72, 0.75, -1)

	// Accent studs sitting proud of the plate front.
	studFront := buttonPlateHT + buttonStudHalf
	for _, s := range buttonStuds {
		center := b.Pos.Add(right.Mul(s[0])).Add(up.Mul(s[1])).Add(b.Facing.Mul(studFront))
		vx, ix = graphics.AppendBox3D(vx, ix, c, center,
			axisAlignedHalf(right, b.Facing, buttonStudHalf, buttonStudHalf, buttonStudHalf),
			buttonStudColor[0], buttonStudColor[1], buttonStudColor[2], -1)
	}

	// Round pressable button, near-white and raised along the normal. When
	// pressed it sinks almost flush with the plate (a subtle nub still poking
	// out) and turns green to signal the active state.
	raise := buttonRaise
	cr, cg, cb := 0.97, 0.97, 0.97
	if b.Pressed {
		raise = 0.015 // nearly flush, barely proud of the plate front
		cr, cg, cb = buttonAccent[0], buttonAccent[1], buttonAccent[2]
	}
	base := b.Pos.Add(b.Facing.Mul(buttonPlateHT))
	vx, ix = graphics.AppendCylinder3D(vx, ix, c, base, b.Facing, buttonRadius, raise, buttonSegs, cr, cg, cb, -1)

	return vx, ix
}
