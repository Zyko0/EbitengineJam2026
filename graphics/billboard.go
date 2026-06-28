package graphics

import (
	"github.com/go-gl/mathgl/mgl64"
	"github.com/hajimehoshi/ebiten/v2"
)

// BillboardCtx holds shared data for projecting puppet-space (u,v) vertices.
type BillboardCtx struct {
	PosRel  mgl64.Vec3 // Foot position relative to camera
	Right   mgl64.Vec3
	Up      mgl64.Vec3
	TF      *mgl64.Mat4 // proj*view
	ScreenW int
	ScreenH int
}

// ProjectUV projects puppet-space (u right, v up from foot) to an ebiten.Vertex.
func (c *BillboardCtx) ProjectUV(u, v, r, g, b float64) (ebiten.Vertex, bool) {
	world := c.PosRel.Add(c.Right.Mul(u)).Add(c.Up.Mul(v))
	clip := c.TF.Mul4x1(world.Vec4(1))
	ndcX := clip.X() / clip.W()
	ndcY := clip.Y() / clip.W()

	return ebiten.Vertex{
		DstX:   float32((1 - ndcX) * 0.5 * float64(c.ScreenW)),
		DstY:   float32((1 - ndcY) * 0.5 * float64(c.ScreenH)),
		ColorR: float32(r),
		ColorG: float32(g),
		ColorB: float32(b),
		ColorA: float32(world.Len()),
	}, clip.W() <= nearW
}

// billboardCorners: TL, TR, BL, BR in (u,v), matching the two-triangle winding
// emitted below.
var billboardCorners = [4]mgl64.Vec2{
	{-0.5, 0.5},
	{0.5, 0.5},
	{-0.5, -0.5},
	{0.5, -0.5},
}

// AppendBillboard appends one camera-facing quad (two triangles) to vx/ix for a
// single entity, ready for the entity-pass DrawTrianglesShader batch.
func AppendBillboard(vx []ebiten.Vertex, ix []uint16, posRel, right, up mgl64.Vec3, w, h float64, tf *mgl64.Mat4, screenW, screenH int, r, g, b, signDist float64) ([]ebiten.Vertex, []uint16) {
	if c := tf.Mul4x1(posRel.Vec4(1)); c.W() <= 0.01 {
		return vx, ix // behind the camera
	}

	base := uint16(len(vx))
	for _, c := range billboardCorners {
		world := posRel.Add(right.Mul(c.X() * w)).Add(up.Mul(c.Y() * h))
		clip := tf.Mul4x1(world.Vec4(1))
		ndcX := clip.X() / clip.W()
		ndcY := clip.Y() / clip.W()
		vx = append(vx, ebiten.Vertex{
			DstX:   float32((1 - ndcX) * 0.5 * float64(screenW)),
			DstY:   float32((1 - ndcY) * 0.5 * float64(screenH)),
			SrcX:   float32(c.X() * 2), // local u in [-1,1], for future shaping
			SrcY:   float32(c.Y() * 2), // local v in [-1,1]
			ColorR: float32(r),
			ColorG: float32(g),
			ColorB: float32(b),
			ColorA: float32(world.Len() * signDist), // camera distance, signed by mode
		})
	}
	ix = append(ix,
		base+0, base+1, base+2,
		base+1, base+2, base+3,
	)

	return vx, ix
}
