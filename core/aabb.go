package core

import (
	"math"

	"github.com/go-gl/mathgl/mgl64"
)

type AABB struct {
	pos, half mgl64.Vec3
}

func NewAABB(position, size mgl64.Vec3) *AABB {
	return &AABB{
		pos:  position,
		half: size.Mul(0.5),
	}
}

func (a *AABB) SetPosition(pos mgl64.Vec3) {
	a.pos = pos
}

func (a *AABB) TestAABB(b *AABB) bool {
	return math.Abs(a.pos[0]-b.pos[0]) < (a.half[0]+b.half[0]) &&
		math.Abs(a.pos[1]-b.pos[1]) < (a.half[1]+b.half[1]) &&
		math.Abs(a.pos[2]-b.pos[2]) < (a.half[2]+b.half[2])
}

func (a *AABB) ResolveAABB_MTV(b *AABB) mgl64.Vec3 {
	aPos := a.pos
	bPos := b.pos
	aSize := a.half
	bSize := b.half

	var NilVec = mgl64.Vec3{0}
	dx := bPos.X() - aPos.X()
	px := (bSize.X() + aSize.X()) - math.Abs(dx)
	if px <= 0 {
		return NilVec
	}

	dy := bPos.Y() - aPos.Y()
	py := (bSize.Y() + aSize.Y()) - math.Abs(dy)
	if py <= 0 {
		return NilVec
	}

	dz := bPos.Z() - aPos.Z()
	pz := (bSize.Z() + aSize.Z()) - math.Abs(dz)
	if pz <= 0 {
		return NilVec
	}

	mtv := mgl64.Vec3{0, 0, 0}
	if px < py && px < pz {
		sx := -1.0
		if math.Signbit(dx) {
			sx = 1
		}
		mtv[0] = px * sx
	} else if py < pz && py < px {
		sy := -1.0
		if math.Signbit(dy) {
			sy = 1
		}
		mtv[1] = py * sy
	} else {
		sz := -1.0
		if math.Signbit(dz) {
			sz = 1
		}
		mtv[2] = pz * sz
	}

	return mtv
}
