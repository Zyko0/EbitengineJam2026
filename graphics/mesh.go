package graphics

import (
	"math"

	"github.com/go-gl/mathgl/mgl64"
	"github.com/hajimehoshi/ebiten/v2"
)

// MeshCtx projects absolute world-space points to entity-pass screen vertices.
type MeshCtx struct {
	CamPos  mgl64.Vec3
	TF      *mgl64.Mat4 // proj*view
	ScreenW int
	ScreenH int
}

// fakeLight gives unlit structures a sense of volume. The entity shader does no
// lighting, so shading is baked into vertex colors: a fixed overhead key light
// plus a camera headlight so faces turned toward the viewer read bright (decals
// mounted head-on stay near their straight color instead of sinking to ambient).
var fakeLight = mgl64.Vec3{0.35, 0.9, 0.25}.Normalize()

const (
	shadeAmbient = 0.45
	shadeKey     = 0.25 // overhead key contribution
	shadeHead    = 0.45 // camera headlight contribution (front-facing surfaces)
)

// shade returns a brightness multiplier for a surface at point with the given
// outward normal, given the camera position in c.
func (c *MeshCtx) shade(point, normal mgl64.Vec3) float64 {
	key := math.Max(0, normal.Dot(fakeLight))
	head := 0.0
	if view := c.CamPos.Sub(point); view.Len() > 1e-6 {
		head = math.Max(0, normal.Dot(view.Normalize()))
	}

	return math.Min(1, shadeAmbient+shadeKey*key+shadeHead*head)
}

// project turns an absolute world point into a screen-space vertex.
func (c *MeshCtx) project(world mgl64.Vec3, r, g, b, signDist float64) ebiten.Vertex {
	rel := world.Sub(c.CamPos)
	clip := c.TF.Mul4x1(rel.Vec4(1))
	ndcX := clip.X() / clip.W()
	ndcY := clip.Y() / clip.W()

	return ebiten.Vertex{
		DstX:   float32((1 - ndcX) * 0.5 * float64(c.ScreenW)),
		DstY:   float32((1 - ndcY) * 0.5 * float64(c.ScreenH)),
		ColorR: float32(r),
		ColorG: float32(g),
		ColorB: float32(b),
		ColorA: float32(rel.Len() * signDist),
	}
}

// nearW is the clip-space w of the near plane
const nearW = 0.01

// clipW returns the clip-space w of a world point
func (c *MeshCtx) clipW(world mgl64.Vec3) float64 {
	return c.TF.Mul4x1(world.Sub(c.CamPos).Vec4(1)).W()
}

// behind reports whether a world point sits at or behind the near plane, where
// projection wraps around
func (c *MeshCtx) behind(world mgl64.Vec3) bool {
	return c.clipW(world) <= nearW
}

// clipNear clips a convex world-space polygon against the near plane
func (c *MeshCtx) clipNear(poly []mgl64.Vec3) []mgl64.Vec3 {
	out := make([]mgl64.Vec3, 0, len(poly)+1)
	n := len(poly)
	for i := 0; i < n; i++ {
		cur, nxt := poly[i], poly[(i+1)%n]
		dc, dn := c.clipW(cur)-nearW, c.clipW(nxt)-nearW
		if dc >= 0 {
			out = append(out, cur)
		}
		if (dc >= 0) != (dn >= 0) { // edge crosses the plane: emit the crossing
			t := dc / (dc - dn)
			out = append(out, cur.Add(nxt.Sub(cur).Mul(t)))
		}
	}

	return out
}

// AppendQuad3D appends one quad from four absolute world-space corners given in
// cyclic order around the perimeter
func AppendQuad3D(vx []ebiten.Vertex, ix []uint16, c *MeshCtx, a, b, cc, d mgl64.Vec3, r, g, bl, signDist float64) ([]ebiten.Vertex, []uint16) {
	corners := [4]mgl64.Vec3{a, b, cc, d}
	poly := c.clipNear(corners[:])
	if len(poly) < 3 {
		return vx, ix
	}

	base := uint16(len(vx))
	for _, p := range poly {
		vx = append(vx, c.project(p, r, g, bl, signDist))
	}
	for i := 1; i+1 < len(poly); i++ {
		ix = append(ix, base, base+uint16(i), base+uint16(i+1))
	}

	return vx, ix
}

// boxFaces lists the four cyclic corner indices of each face, paired with its
// outward axis normal. Corners are the 8 box vertices (see AppendBox3D).
var boxFaces = [6]struct {
	idx    [4]int
	normal mgl64.Vec3
}{
	{[4]int{0, 1, 2, 3}, mgl64.Vec3{0, -1, 0}}, // bottom
	{[4]int{4, 5, 6, 7}, mgl64.Vec3{0, 1, 0}},  // top
	{[4]int{0, 1, 5, 4}, mgl64.Vec3{0, 0, -1}}, // -z
	{[4]int{3, 2, 6, 7}, mgl64.Vec3{0, 0, 1}},  // +z
	{[4]int{0, 3, 7, 4}, mgl64.Vec3{-1, 0, 0}}, // -x
	{[4]int{1, 2, 6, 5}, mgl64.Vec3{1, 0, 0}},  // +x
}

// AppendBox3D appends an axis-aligned box centered at center with the given half
// extents. Faces pointing away from the camera are culled (so a convex box never
// overdraws itself), and each visible face is fake-lit for depth.
func AppendBox3D(vx []ebiten.Vertex, ix []uint16, c *MeshCtx, center, half mgl64.Vec3, r, g, b, signDist float64) ([]ebiten.Vertex, []uint16) {
	hx, hy, hz := half.X(), half.Y(), half.Z()
	corner := [8]mgl64.Vec3{
		center.Add(mgl64.Vec3{-hx, -hy, -hz}),
		center.Add(mgl64.Vec3{hx, -hy, -hz}),
		center.Add(mgl64.Vec3{hx, -hy, hz}),
		center.Add(mgl64.Vec3{-hx, -hy, hz}),
		center.Add(mgl64.Vec3{-hx, hy, -hz}),
		center.Add(mgl64.Vec3{hx, hy, -hz}),
		center.Add(mgl64.Vec3{hx, hy, hz}),
		center.Add(mgl64.Vec3{-hx, hy, hz}),
	}
	for _, f := range boxFaces {
		fc := corner[f.idx[0]].Add(corner[f.idx[2]]).Mul(0.5) // face center
		if f.normal.Dot(c.CamPos.Sub(fc)) <= 0 {
			continue // back face
		}
		s := c.shade(fc, f.normal)
		vx, ix = AppendQuad3D(vx, ix, c,
			corner[f.idx[0]], corner[f.idx[1]], corner[f.idx[2]], corner[f.idx[3]],
			r*s, g*s, b*s, signDist)
	}

	return vx, ix
}

// basis returns two unit vectors spanning the plane perpendicular to n.
func basis(n mgl64.Vec3) (u, v mgl64.Vec3) {
	ref := mgl64.Vec3{0, 1, 0}
	if math.Abs(n.Y()) > 0.9 {
		ref = mgl64.Vec3{1, 0, 0}
	}
	u = ref.Cross(n).Normalize()
	v = n.Cross(u).Normalize()

	return u, v
}

// AppendDisc3D appends a filled circle (triangle fan) centered at center, facing
// normal, with the given radius and segment count
func AppendDisc3D(vx []ebiten.Vertex, ix []uint16, c *MeshCtx, center, normal mgl64.Vec3, radius float64, segments int, r, g, b, signDist float64) ([]ebiten.Vertex, []uint16) {
	u, v := basis(normal)
	s := c.shade(center, normal)
	r, g, b = r*s, g*s, b*s
	if c.behind(center) {
		return vx, ix
	}

	ring := make([]mgl64.Vec3, segments)
	for i := range ring {
		a := float64(i) / float64(segments) * 2 * math.Pi
		ring[i] = center.Add(u.Mul(radius * math.Cos(a))).Add(v.Mul(radius * math.Sin(a)))
		if c.behind(ring[i]) {
			return vx, ix
		}
	}
	centerIdx := uint16(len(vx))
	vx = append(vx, c.project(center, r, g, b, signDist))
	for _, p := range ring {
		vx = append(vx, c.project(p, r, g, b, signDist))
	}
	for i := 0; i < segments; i++ {
		ix = append(ix, centerIdx, centerIdx+uint16(i)+1, centerIdx+uint16((i+1)%segments)+1)
	}

	return vx, ix
}

// AppendCylinder3D appends a capped cylinder from baseCenter extending height
// along axis (unit), with the given radius and segment count
func AppendCylinder3D(vx []ebiten.Vertex, ix []uint16, c *MeshCtx, baseCenter, axis mgl64.Vec3, radius, height float64, segments int, r, g, b, signDist float64) ([]ebiten.Vertex, []uint16) {
	u, v := basis(axis)
	top := baseCenter.Add(axis.Mul(height))
	for i := 0; i < segments; i++ {
		a0 := float64(i) / float64(segments) * 2 * math.Pi
		a1 := float64(i+1) / float64(segments) * 2 * math.Pi
		d0 := u.Mul(math.Cos(a0)).Add(v.Mul(math.Sin(a0)))
		d1 := u.Mul(math.Cos(a1)).Add(v.Mul(math.Sin(a1)))
		mid := d0.Add(d1).Normalize()
		facet := baseCenter.Add(axis.Mul(height / 2)).Add(mid.Mul(radius))
		s := c.shade(facet, mid)
		vx, ix = AppendQuad3D(vx, ix, c,
			baseCenter.Add(d0.Mul(radius)),
			baseCenter.Add(d1.Mul(radius)),
			top.Add(d1.Mul(radius)),
			top.Add(d0.Mul(radius)),
			r*s, g*s, b*s, signDist)
	}

	return AppendDisc3D(vx, ix, c, top, axis, radius, segments, r, g, b, signDist)
}
