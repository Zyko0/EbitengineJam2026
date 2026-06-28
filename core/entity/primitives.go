package entity

import (
	"math"

	"github.com/Zyko0/EbitengineJam2026/graphics"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/hajimehoshi/ebiten/v2"
)

// appendCapsule draws a capsule from a to b
func appendCapsule(vx []ebiten.Vertex, ix []uint16, ctx *graphics.BillboardCtx,
	a, b mgl64.Vec2, ra, rb, cr, cg, cb float64) ([]ebiten.Vertex, []uint16) {
	dx := b[0] - a[0]
	dy := b[1] - a[1]
	l := math.Sqrt(dx*dx + dy*dy)
	if l < 1e-6 {
		return vx, ix
	}
	
	dU, dV := dx/l, dy/l
	pU, pV := -dV, dU

	base := uint16(len(vx))
	v0, b0 := ctx.ProjectUV(a[0]+pU*ra, a[1]+pV*ra, cr, cg, cb)
	v1, b1 := ctx.ProjectUV(b[0]+pU*rb, b[1]+pV*rb, cr, cg, cb)
	v2, b2 := ctx.ProjectUV(a[0]-pU*ra, a[1]-pV*ra, cr, cg, cb)
	v3, b3 := ctx.ProjectUV(b[0]-pU*rb, b[1]-pV*rb, cr, cg, cb)
	vx = append(vx, v0, v1, v2, v3)
	if !b0 && !b1 && !b2 {
		ix = append(ix, base, base+1, base+2)
	}
	if !b1 && !b3 && !b2 {
		ix = append(ix, base+1, base+3, base+2)
	}

	// Rounded caps: sweep a half circle from each end, tangent/normal directions
	// flipped so both bulge outward.
	vx, ix = appendCap(vx, ix, ctx, a, ra, pU, pV, -dU, -dV, cr, cg, cb)
	vx, ix = appendCap(vx, ix, ctx, b, rb, -pU, -pV, dU, dV, cr, cg, cb)
	
	return vx, ix
}

// appendCap emits a rounded half-disc of radius r centred at c
func appendCap(vx []ebiten.Vertex, ix []uint16, ctx *graphics.BillboardCtx,
	c mgl64.Vec2, r, eU, eV, fU, fV, cr, cg, cb float64) ([]ebiten.Vertex, []uint16) {
	base := uint16(len(vx))
	center, bc := ctx.ProjectUV(c[0], c[1], cr, cg, cb)
	vx = append(vx, center)
	prevBehind := true // no ring vertex emitted yet

	const N = 4
	for i := 0; i <= N; i++ {
		ang := float64(i) / N * math.Pi
		cu := c[0] + r*(eU*math.Cos(ang)+fU*math.Sin(ang))
		cv := c[1] + r*(eV*math.Cos(ang)+fV*math.Sin(ang))
		v, behind := ctx.ProjectUV(cu, cv, cr, cg, cb)
		vx = append(vx, v)
		if i > 0 && !bc && !prevBehind && !behind {
			ix = append(ix, base, base+uint16(i), base+uint16(i+1))
		}
		prevBehind = behind
	}

	return vx, ix
}

// axisAlignedHalf maps local half-extents (hr along the right axis, hy along
// world-up, hf along the forward axis) into world-space half-extents for
// graphics.AppendBox3D. right and forward must be axis-aligned unit vectors, so
// each contributes to exactly one of the world x/z components.
func axisAlignedHalf(right, forward mgl64.Vec3, hr, hy, hf float64) mgl64.Vec3 {
	return mgl64.Vec3{
		hr*math.Abs(right.X()) + hf*math.Abs(forward.X()),
		hy,
		hr*math.Abs(right.Z()) + hf*math.Abs(forward.Z()),
	}
}

func appendEllipse(vx []ebiten.Vertex, ix []uint16, ctx *graphics.BillboardCtx,
	cu, cv, rx, ry float64, n int, cr, cg, cb float64) ([]ebiten.Vertex, []uint16) {
	base := uint16(len(vx))
	center, bc := ctx.ProjectUV(cu, cv, cr, cg, cb)
	vx = append(vx, center)
	prevBehind := true // no ring vertex emitted yet
	for i := 0; i <= n; i++ {
		a := float64(i) / float64(n) * 2 * math.Pi
		v, behind := ctx.ProjectUV(cu+rx*math.Sin(a), cv+ry*math.Cos(a), cr, cg, cb)
		vx = append(vx, v)
		if i > 0 && !bc && !prevBehind && !behind {
			ix = append(ix, base, base+uint16(i), base+uint16(i+1))
		}
		prevBehind = behind
	}

	return vx, ix
}
