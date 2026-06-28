package level

import "github.com/go-gl/mathgl/mgl64"

// dfsEulerTour visits all rooms in allowed reachable from start via DFS,
// recording each room on entry and each backtrack step. Consecutive entries
// are room-graph-adjacent. The returned slice represents a closed circuit:
// the wrap-around from the last element back to the first closes the loop.
func (g *gen) dfsEulerTour(start int, allowed map[int]bool) []int {
	visited := make(map[int]bool)
	var tour []int
	var dfs func(c int)
	dfs = func(c int) {
		visited[c] = true
		tour = append(tour, c)
		nbs := g.neighbours(c)
		g.rng.Shuffle(len(nbs), func(i, j int) { nbs[i], nbs[j] = nbs[j], nbs[i] })
		for _, nb := range nbs {
			if !allowed[nb] || visited[nb] {
				continue
			}
			dfs(nb)
			tour = append(tour, c) // backtrack
		}
	}
	dfs(start)
	// The DFS always ends with start (final backtrack to root). Trim it so the
	// wrap-around in tourToWaypoints closes the loop without a zero-length leg.
	if len(tour) > 1 {
		return tour[:len(tour)-1]
	}
	return tour
}

// tourToWaypoints converts a room-index tour to world-space waypoints.
// wall=true builds a wall-hugging dense path; wall=false builds a direct
// diagonal-friendly path with line-of-sight simplification.
func (g *gen) tourToWaypoints(tour []int, lvl *Level, wall bool) []mgl64.Vec3 {
	if len(tour) < 2 {
		return nil
	}
	var pts []mgl64.Vec3
	for i := range tour {
		a, b := tour[i], tour[(i+1)%len(tour)]
		ax, az := g.nearestOpen(RoomSize*(a%g.gw)+8, RoomSize*(a/g.gw)+8)
		bx, bz := g.nearestOpen(RoomSize*(b%g.gw)+8, RoomSize*(b/g.gw)+8)
		var path [][2]int
		if wall {
			path = lvl.WallHugBFSPath(ax, az, bx, bz)
		} else {
			path = lvl.DiagBFSPath(ax, az, bx, bz)
		}
		if path == nil {
			path = lvl.BFSPath(ax, az, bx, bz)
		}
		if path == nil {
			return nil
		}
		if wall {
			for j := 0; j < len(path)-1; j++ {
				pts = append(pts, mgl64.Vec3{float64(path[j][0]) + 0.5, 0, float64(path[j][1]) + 0.5})
			}
		} else {
			simplified := simplifyPath(lvl, path)
			for j := 0; j < len(simplified)-1; j++ {
				pts = append(pts, simplified[j])
			}
		}
	}
	return pts
}

// simplifyPath reduces a dense BFS cell path to sparse waypoints using
// line-of-sight shortcutting.
func simplifyPath(lvl *Level, path [][2]int) []mgl64.Vec3 {
	if len(path) == 0 {
		return nil
	}
	cv := func(c [2]int) mgl64.Vec3 { return mgl64.Vec3{float64(c[0]) + 0.5, 0, float64(c[1]) + 0.5} }
	out := []mgl64.Vec3{cv(path[0])}
	anchor := 0
	for i := 1; i < len(path); i++ {
		if !lvl.LineClear(cv(path[anchor]), cv(path[i])) {
			out = append(out, cv(path[i-1]))
			anchor = i - 1
		}
	}
	if len(path) > 1 {
		out = append(out, cv(path[len(path)-1]))
	}
	return out
}

// buildCircuits partitions all rooms into two disjoint connected components
// via a balanced spanning-tree cut, then builds a DFS Euler tour for each
// so every room in a component is visited. The wall-hug circuit covers rooms
// near spawn; the direct circuit covers the far half.
//
// Returns world-space waypoints and the room-index slices for both circuits.
func (g *gen) buildCircuits(lvl *Level) (wallPts, directPts []mgl64.Vec3, wallCycle, directCycle []int) {
	n := g.gw * g.gz

	// BFS spanning tree from spawn (room 0).
	parent := make([]int, n)
	for i := range parent {
		parent[i] = -1
	}
	parent[0] = 0
	bfsOrder := make([]int, 0, n)
	bfsOrder = append(bfsOrder, 0)
	q := []int{0}
	for len(q) > 0 {
		c := q[0]
		q = q[1:]
		for _, nb := range g.neighbours(c) {
			if parent[nb] != -1 {
				continue
			}
			parent[nb] = c
			bfsOrder = append(bfsOrder, nb)
			q = append(q, nb)
		}
	}
	if len(bfsOrder) < 2 {
		return
	}

	// Subtree sizes: process leaves first (reverse BFS order).
	subSize := make([]int, n)
	for _, c := range bfsOrder {
		subSize[c] = 1
	}
	for i := len(bfsOrder) - 1; i > 0; i-- {
		c := bfsOrder[i]
		subSize[parent[c]] += subSize[c]
	}

	// Find the spanning-tree edge (parent[cutRoot], cutRoot) whose removal
	// splits the tree most evenly (subSize[cutRoot] closest to total/2).
	total := len(bfsOrder)
	cutRoot := bfsOrder[1]
	bestDiff := abs(subSize[cutRoot] - (total - subSize[cutRoot]))
	for i := 2; i < len(bfsOrder); i++ {
		c := bfsOrder[i]
		if d := abs(subSize[c] - (total - subSize[c])); d < bestDiff {
			bestDiff = d
			cutRoot = c
		}
	}

	// Mark rooms in cutRoot's subtree -> direct circuit; rest -> wall circuit.
	inSubtree := make([]bool, n)
	inSubtree[cutRoot] = true
	for i := 1; i < len(bfsOrder); i++ {
		c := bfsOrder[i]
		if inSubtree[parent[c]] {
			inSubtree[c] = true
		}
	}

	wallAllowed := make(map[int]bool)
	directAllowed := make(map[int]bool)
	for _, c := range bfsOrder {
		if inSubtree[c] {
			directAllowed[c] = true
		} else {
			wallAllowed[c] = true
		}
	}

	// Euler tours: every room in each component is visited at least once.
	wallCycle = g.dfsEulerTour(0, wallAllowed)
	directCycle = g.dfsEulerTour(cutRoot, directAllowed)

	wallPts = g.tourToWaypoints(wallCycle, lvl, true)
	directPts = g.tourToWaypoints(directCycle, lvl, false)
	return
}

// neighbours lists room indices reachable from c through an open edge.
func (g *gen) neighbours(c int) []int {
	var out []int
	if g.edge[c]&EN != 0 {
		out = append(out, c-g.gw)
	}
	if g.edge[c]&ES != 0 {
		out = append(out, c+g.gw)
	}
	if g.edge[c]&EW != 0 {
		out = append(out, c-1)
	}
	if g.edge[c]&EE != 0 {
		out = append(out, c+1)
	}
	return out
}
