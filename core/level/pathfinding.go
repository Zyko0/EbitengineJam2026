package level

import (
	"math"

	"github.com/go-gl/mathgl/mgl64"
)

// LineClear reports whether the straight XZ segment a->b crosses no solid cell,
// using the same grid DDA the shaders walk
func (l *Level) LineClear(a, b mgl64.Vec3) bool {
	x0, z0 := a.X(), a.Z()
	dx, dz := b.X()-x0, b.Z()-z0
	cx, cz := int(math.Floor(x0)), int(math.Floor(z0))
	tx, tz := int(math.Floor(b.X())), int(math.Floor(b.Z()))

	stepX, stepZ := 1, 1
	if dx < 0 {
		stepX = -1
	}
	if dz < 0 {
		stepZ = -1
	}
	tMaxX, tMaxZ := math.Inf(1), math.Inf(1)
	tDeltaX, tDeltaZ := math.Inf(1), math.Inf(1)
	if dx != 0 {
		nextX := float64(cx)
		if stepX > 0 {
			nextX = float64(cx + 1)
		}
		tMaxX = (nextX - x0) / dx
		tDeltaX = math.Abs(1 / dx)
	}
	if dz != 0 {
		nextZ := float64(cz)
		if stepZ > 0 {
			nextZ = float64(cz + 1)
		}
		tMaxZ = (nextZ - z0) / dz
		tDeltaZ = math.Abs(1 / dz)
	}
	for {
		if cx == tx && cz == tz {
			return true
		}
		if tMaxX < tMaxZ {
			tMaxX += tDeltaX
			cx += stepX
		} else {
			tMaxZ += tDeltaZ
			cz += stepZ
		}
		if l.Solid(cx, cz) {
			return false
		}
		if tMaxX > 1 && tMaxZ > 1 {
			return true // passed b without a hit
		}
	}
}

// BFSPath returns the shortest 4-connected open-cell path from (sx,sz) to
// (tx,tz) inclusive, or nil if none.
func (l *Level) BFSPath(sx, sz, tx, tz int) [][2]int {
	if l.Solid(sx, sz) || l.Solid(tx, tz) {
		return nil
	}
	prev := make([]int, l.width*l.depth)
	for i := range prev {
		prev[i] = -2
	}
	start := sx + sz*l.width
	prev[start] = -1
	q := []int{start}
	dirs := [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}
	for len(q) > 0 {
		c := q[0]
		q = q[1:]
		cx, cz := c%l.width, c/l.width
		if cx == tx && cz == tz {
			break
		}
		for _, d := range dirs {
			nx, nz := cx+d[0], cz+d[1]
			if l.Solid(nx, nz) {
				continue
			}
			ni := nx + nz*l.width
			if prev[ni] != -2 {
				continue
			}
			prev[ni] = c
			q = append(q, ni)
		}
	}
	ti := tx + tz*l.width
	if prev[ti] == -2 {
		return nil
	}
	var path [][2]int
	for c := ti; c != -1; c = prev[c] {
		path = append([][2]int{{c % l.width, c / l.width}}, path...)
	}
	return path
}

// WallHugBFSPath finds the lowest-cost path from (sx,sz) to (tx,tz) where
// wall-adjacent cells cost 1 and open cells cost 2, biasing the route to hug walls.
// Uses Dial's algorithm (3-bucket circular queue) for O(V+E) time.
func (l *Level) WallHugBFSPath(sx, sz, tx, tz int) [][2]int {
	if l.Solid(sx, sz) || l.Solid(tx, tz) {
		return nil
	}
	n := l.width * l.depth
	const inf = 1<<31 - 1
	dist := make([]int, n)
	for i := range dist {
		dist[i] = inf
	}
	prev := make([]int, n)
	for i := range prev {
		prev[i] = -2
	}

	hasWallNeighbor := func(x, z int) bool {
		return l.Solid(x+1, z) || l.Solid(x-1, z) || l.Solid(x, z+1) || l.Solid(x, z-1)
	}

	start := sx + sz*l.width
	dist[start] = 0
	prev[start] = -1
	// 3 buckets: costs are 1 or 2, so active window is at most 3 wide.
	const nb = 3
	var buckets [nb][]int
	buckets[0] = append(buckets[0], start)
	dirs := [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}
	target := tx + tz*l.width

	found := false
	for d := 0; d <= 2*n && !found; d++ {
		b := buckets[d%nb]
		buckets[d%nb] = nil
		for _, u := range b {
			if dist[u] != d {
				continue
			}
			if u == target {
				found = true
				break
			}
			ux, uz := u%l.width, u/l.width
			for _, dir := range dirs {
				nx, nz := ux+dir[0], uz+dir[1]
				if l.Solid(nx, nz) {
					continue
				}
				cost := 2
				if hasWallNeighbor(nx, nz) {
					cost = 1
				}
				ni := nx + nz*l.width
				nd := d + cost
				if nd < dist[ni] {
					dist[ni] = nd
					prev[ni] = u
					buckets[nd%nb] = append(buckets[nd%nb], ni)
				}
			}
		}
	}

	if prev[target] == -2 {
		return nil
	}
	var path [][2]int
	for c := target; c != -1; c = prev[c] {
		path = append([][2]int{{c % l.width, c / l.width}}, path...)
	}
	return path
}

// DiagBFSPath returns the shortest 8-connected path from (sx,sz) to (tx,tz).
// Diagonal moves are only taken when both adjacent orthogonal cells are open,
// preventing corner-clipping.
func (l *Level) DiagBFSPath(sx, sz, tx, tz int) [][2]int {
	if l.Solid(sx, sz) || l.Solid(tx, tz) {
		return nil
	}
	prev := make([]int, l.width*l.depth)
	for i := range prev {
		prev[i] = -2
	}
	start := sx + sz*l.width
	prev[start] = -1
	q := []int{start}
	dirs := [8][2]int{
		{1, 0}, {-1, 0}, {0, 1}, {0, -1},
		{1, 1}, {1, -1}, {-1, 1}, {-1, -1},
	}
	target := tx + tz*l.width
	for len(q) > 0 {
		c := q[0]
		q = q[1:]
		if c == target {
			break
		}
		cx, cz := c%l.width, c/l.width
		for _, d := range dirs {
			nx, nz := cx+d[0], cz+d[1]
			if l.Solid(nx, nz) {
				continue
			}
			if d[0] != 0 && d[1] != 0 && (l.Solid(cx+d[0], cz) || l.Solid(cx, cz+d[1])) {
				continue
			}
			ni := nx + nz*l.width
			if prev[ni] != -2 {
				continue
			}
			prev[ni] = c
			q = append(q, ni)
		}
	}
	if prev[target] == -2 {
		return nil
	}
	var path [][2]int
	for c := target; c != -1; c = prev[c] {
		path = append([][2]int{{c % l.width, c / l.width}}, path...)
	}
	return path
}

// InCone reports whether target lies within a view cone of half-angle acos(cosHalf)
// around fwd from the eye (XZ plane). Pair with LineClear for sight tests.
func InCone(eye, fwd, target mgl64.Vec3, cosHalf float64) bool {
	to := mgl64.Vec3{target.X() - eye.X(), 0, target.Z() - eye.Z()}
	if to.Len() < 1e-6 {
		return true
	}
	f := mgl64.Vec3{fwd.X(), 0, fwd.Z()}
	if f.Len() < 1e-6 {
		return false
	}
	return to.Normalize().Dot(f.Normalize()) >= cosHalf
}
