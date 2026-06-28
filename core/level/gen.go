package level

import (
	"math"
	"math/rand"

	"github.com/go-gl/mathgl/mgl64"
)

const (
	spawnEyeY = 2.0  // player center height (matches core physics floor)
	plateThk  = 0.06 // button plate half-thickness (matches entity.Button)
	elevHalf  = 1.0  // elevator platform half-footprint (matches entity.Elevator)

	// maxSightline caps how far an unbroken 3-wide open stretch may run before the
	// deform pass walls it (≈1.5 rooms): short enough that distant rooms, enemies
	// and objectives stay hidden until you round the next divider.
	maxSightline = 24
)

// Placement is a budgeted silhouette to spawn: an archetype at a world position.
type Placement struct {
	Pos mgl64.Vec3
	Dir mgl64.Vec3
}

// Manifest is everything the EntityManager needs to populate a generated level:
// the player start, the injected objective anchors, the Spinner loop, and the
// ring-distributed enemy/event placements (§6, §9).
type Manifest struct {
	Spawn         mgl64.Vec3
	SpawnYaw      float64 // camera yaw facing the first open door in the spawn room
	ButtonPos     mgl64.Vec3
	ButtonFacing  mgl64.Vec3
	ElevatorPos   mgl64.Vec3
	ElevatorFwd   mgl64.Vec3
	SpiderSpawn   mgl64.Vec3 // open ground cell the lone ceiling spider starts above
	Circuit       []mgl64.Vec3
	DirectCircuit []mgl64.Vec3
	Enemies       []Placement
	Events        []Placement
}

// gen holds the mutable state of one generation attempt: the room grid (edges /
// usage / chosen templates / doors / decoration) and the cell grid being carved.
type gen struct {
	rng    *rand.Rand
	gw, gz int    // room grid dimensions
	w, d   int    // cell grid dimensions
	edge   []Edge // open edges per room (world orientation)
	used   []bool
	tmpl   []int       // chosen template index per room (-1 = plain open hall)
	rot    []int       // template rotation per room
	door   [][2]int    // per room*4+dir door span [lo,hi] in local cells (-1 = none)
	decor  []decorKind // phase-B interior pattern per room (decorNone = plain)
	walls  []bool
	lib    []Room
}

// Generate builds a full-budget level and its Manifest from a seed and difficulty
func Generate(seed int64, difficulty int, lib []Room) (*Level, *Manifest) {
	var last *Level
	var lastM *Manifest

	for attempt := range 12 {
		g := newGen(rand.New(rand.NewSource(seed+int64(attempt))), difficulty, lib)
		lvl, m := g.run(difficulty)
		last, lastM = lvl, m
		if g.valid(m) {
			return lvl, m
		}
	}

	return last, lastM // connectivity is guaranteed by construction; safety net
}

func newGen(rng *rand.Rand, _ int, lib []Room) *gen {
	// A single 4x4 footprint fills the 128-int wall budget (§2). Identity and
	// pacing come from per-edge door widths/offsets and a deform pass, not the
	// macro shape: dropping the old long 2x8 shaft removes the multi-room straight
	// sightlines that spoiled distant entities and objectives.
	gw, gz := 4, 4
	n := gw * gz
	g := &gen{
		rng: rng, gw: gw, gz: gz, w: gw * RoomSize, d: gz * RoomSize,
		edge: make([]Edge, n), used: make([]bool, n),
		tmpl: make([]int, n), rot: make([]int, n),
		walls: make([]bool, gw*RoomSize*gz*RoomSize), lib: lib,
	}
	for i := range g.walls {
		g.walls[i] = true
	}

	return g
}

func (g *gen) idx(rx, rz int) int { return rz*g.gw + rx }

// roomCenter is the world center of a room (entity ground, y=0).
func (g *gen) roomCenter(rx, rz int) mgl64.Vec3 {
	return mgl64.Vec3{
		float64(RoomSize*rx + 8),
		0,
		float64(RoomSize*rz + 8),
	}
}

func (g *gen) run(difficulty int) (*Level, *Manifest) {
	spawn := g.idx(0, 0)
	deep := g.idx(g.gw-1, g.gz-1) // notional far end, only used to bias the button detour

	g.carveMaze(0, 0)
	g.braid(difficulty)
	g.chooseTemplates(difficulty)
	g.assignDoors()
	g.stamp()

	route := g.routeRooms(spawn, deep)
	button := g.pickButtonRoom(route, spawn, deep)
	elev := g.pickElevatorRoom(spawn, button)

	bpos, bfacing := g.injectButton(button)
	epos, efwd := g.injectElevator(elev)

	g.decorate(spawn, button, elev)
	g.breakSightlines(maxSightline)

	lvl := NewLevel(g.w, g.d, g.walls)
	lvl.circuit, lvl.directCircuit, _, _ = g.buildCircuits(lvl)

	c := g.roomCenter(0, 0)
	m := &Manifest{
		Spawn:         mgl64.Vec3{c.X(), spawnEyeY, c.Z()},
		SpawnYaw:      spawnFacingYaw(g.edge[spawn]),
		ButtonPos:     bpos,
		ButtonFacing:  bfacing,
		ElevatorPos:   epos,
		ElevatorFwd:   efwd,
		SpiderSpawn:   g.placePos(deep), // lone spider starts on the ceiling far from spawn
		Circuit:       lvl.circuit,
		DirectCircuit: lvl.directCircuit,
	}

	return lvl, m
}

// carveMaze runs a randomized-DFS backtracker over the room grid, opening doors
// between visited neighbours so every room ends up connected (§6 step 3).
func (g *gen) carveMaze(sx, sz int) {
	type cell struct{ x, z int }
	stack := []cell{{sx, sz}}
	g.used[g.idx(sx, sz)] = true
	for len(stack) > 0 {
		c := stack[len(stack)-1]
		order := g.rng.Perm(4)
		advanced := false
		for _, oi := range order {
			dd := connDirs[oi]
			nx, nz := c.x+dd.dx, c.z+dd.dz
			if nx < 0 || nx >= g.gw || nz < 0 || nz >= g.gz || g.used[g.idx(nx, nz)] {
				continue
			}
			g.edge[g.idx(c.x, c.z)] |= dd.e
			g.edge[g.idx(nx, nz)] |= dd.opp
			g.used[g.idx(nx, nz)] = true
			stack = append(stack, cell{nx, nz})
			advanced = true
			break
		}
		if !advanced {
			stack = stack[:len(stack)-1]
		}
	}
}

// connDirs is the four orthogonal neighbour steps with their edge bit and the
// reciprocal bit on the neighbour, used to open adjacencies.
var connDirs = [4]struct {
	dx, dz int
	e, opp Edge
}{{0, -1, EN, ES}, {1, 0, EE, EW}, {0, 1, ES, EN}, {-1, 0, EW, EE}}

// connect opens the adjacency from room (rx,rz) in direction d if the neighbour
// is in-bounds and not already joined; returns whether a new door was added.
func (g *gen) connect(rx, rz, d int) bool {
	dd := connDirs[d]
	nx, nz := rx+dd.dx, rz+dd.dz
	if nx < 0 || nx >= g.gw || nz < 0 || nz >= g.gz {
		return false
	}
	a, b := g.idx(rx, rz), g.idx(nx, nz)
	if g.edge[a]&dd.e != 0 {
		return false
	}
	g.edge[a] |= dd.e
	g.edge[b] |= dd.opp
	return true
}

// braid densifies the spanning tree into an interconnected maze (§6 step 5):
// it removes nearly all dead ends (so rooms gain doors and pick the open,
// multi-door templates) and adds a budget of extra loops for alternative routes
// and the patrol cycle. The result reads as linked halls and corridors rather
// than a sparse tree threading thin lanes through mostly-solid blocks.
func (g *gen) braid(difficulty int) {
	n := g.gw * g.gz
	// 1) Dead-end removal: every degree-1 room earns a second door. Keep one
	//    intentional dead end for dead-end-tagged events.
	keep := 1
	var deads []int
	for c := 0; c < n; c++ {
		if g.used[c] && g.edge[c].Count() == 1 {
			deads = append(deads, c)
		}
	}
	g.rng.Shuffle(len(deads), func(i, j int) { deads[i], deads[j] = deads[j], deads[i] })
	for i, c := range deads {
		if i < keep {
			continue
		}
		rx, rz := c%g.gw, c/g.gw
		for _, d := range g.rng.Perm(4) {
			if g.connect(rx, rz, d) {
				break
			}
		}
	}
	// 2) Extra loops: a small budget for reroutes and a real patrol cycle. Kept
	//    minimal so the corridor backbone survives (a maze, not an open grid):
	//    dead-end removal alone already braids leaves into through-corridors.
	extra := 2 + difficulty
	for t, added := 0, 0; added < extra && t < n*8; t++ {
		rx, rz := g.rng.Intn(g.gw), g.rng.Intn(g.gz)
		if g.connect(rx, rz, g.rng.Intn(4)) {
			added++
		}
	}
}

type tcand struct{ ri, rot int }

// chooseTemplates assigns each used room a catalogue template whose edge set
// matches (under rotation), weighted-random for variety (§6 step 6). Spacious
// open junctions (3-/4-door halls) are capped per layout so most intersections
// read as tight corridor T/cross shapes that keep walls and short sightlines;
// big open rooms stay rare landmarks.
func (g *gen) chooseTemplates(difficulty int) {
	bigOpen := 3 + difficulty // budget of spacious open chambers per level
	for c := 0; c < g.gw*g.gz; c++ {
		if !g.used[c] {
			continue
		}
		we := g.edge[c]
		var pool []tcand
		for ri := range g.lib {
			if bigOpen <= 0 && g.lib[ri].Tags.Has(TagHall) {
				continue // out of open-chamber budget: force a corridor/bend/junction
			}
			for rot := 0; rot < 4; rot++ {
				if g.lib[ri].Edges.Rotate(rot) == we {
					pool = append(pool, tcand{ri, rot})
					break
				}
			}
		}
		if len(pool) == 0 {
			g.tmpl[c] = -1 // no match: fall back to a plain open hall
			continue
		}
		pick := g.weightedPick(pool)
		g.tmpl[c], g.rot[c] = pick.ri, pick.rot
		if g.lib[pick.ri].Tags.Has(TagHall) {
			bigOpen--
		}
	}
}

func (g *gen) weightedPick(pool []tcand) tcand {
	total := 0
	for _, p := range pool {
		total += g.weight(p.ri)
	}
	n := g.rng.Intn(total)
	for _, p := range pool {
		if n < g.weight(p.ri) {
			return p
		}
		n -= g.weight(p.ri)
	}
	return pool[0]
}

func (g *gen) weight(ri int) int {
	if w := g.lib[ri].Weight; w > 0 {
		return w
	}
	return 1
}

// tags returns a room's effective tags (its template's, or Hall for plain fill).
func (g *gen) tags(c int) Tag {
	if g.tmpl[c] < 0 {
		return TagHall
	}
	return g.lib[g.tmpl[c]].Tags
}

// stamp carves every used room's interior, then the centered 4-wide door slots
// (§6 step 7).
func (g *gen) stamp() {
	for c := 0; c < g.gw*g.gz; c++ {
		if !g.used[c] {
			continue
		}
		s := &RoomStamp{walls: g.walls, mapW: g.w, ox: RoomSize * (c % g.gw), oz: RoomSize * (c / g.gw), rot: g.rot[c]}
		if g.tmpl[c] < 0 {
			s.OpenRect(1, 1, 14, 14)
		} else {
			g.lib[g.tmpl[c]].Carve(s)
		}
	}
	g.carveDoors()
}

// openRoom reports whether room c's template is a full open chamber (reaches
// every edge cell), so its doors may be offset off-centre and the deform pass
// may stamp a pattern inside. Corridors, bends and the small specials keep their
// doors centred in their lane.
func (g *gen) openRoom(c int) bool {
	if g.tmpl[c] < 0 {
		return true // plain fill is an open square
	}
	return g.lib[g.tmpl[c]].Tags.Has(TagHall)
}

// assignDoors picks a span for every open edge, shared by the two rooms it joins
// so both carve the same slot (§2.1). Widths vary (narrow doorways to wide
// openings) and, when both sides are open chambers, the slot is pushed off-centre
// so entrances no longer line up across rooms. That breaks the uniform 6-wide
// aligned tubes that let players see across several rooms at once.
func (g *gen) assignDoors() {
	n := g.gw * g.gz
	g.door = make([][2]int, n*4)
	for i := range g.door {
		g.door[i] = [2]int{-1, -1}
	}
	for c := 0; c < n; c++ {
		if !g.used[c] {
			continue
		}
		for di := 0; di < 4; di++ {
			dd := connDirs[di]
			if g.edge[c]&dd.e == 0 {
				continue
			}
			nb := c + dd.dz*g.gw + dd.dx
			if nb < c {
				continue // assign each shared edge once, from the lower-indexed room
			}
			lo, hi := g.rollDoor(g.openRoom(c) && g.openRoom(nb))
			g.door[c*4+di] = [2]int{lo, hi}
			g.door[nb*4+(di+2)%4] = [2]int{lo, hi}
		}
	}
}

// rollDoor rolls a door span. Centred spans stay inside the doorLo..doorHi lane
// every template covers; when both rooms are open chambers most doors are pushed
// to a random off-centre position (keeping the full span on the edge).
func (g *gen) rollDoor(offsetOK bool) (lo, hi int) {
	band := [...]int{2, 2, 3, 3, 3, 4, 4, 5, 6}
	w := band[g.rng.Intn(len(band))]
	if offsetOK && g.rng.Intn(3) > 0 { // ~2/3 of eligible doors go off-centre
		lo = 1 + g.rng.Intn(RoomSize-1-w) // span stays within cells 1..14
		return lo, lo + w - 1
	}
	lo = doorLo + (doorHi-doorLo+1-w)/2 // centred inside the safe lane band
	return lo, lo + w - 1
}

func (g *gen) carveDoors() {
	for c := 0; c < g.gw*g.gz; c++ {
		if !g.used[c] {
			continue
		}
		ox, oz := RoomSize*(c%g.gw), RoomSize*(c/g.gw)
		for di := 0; di < 4; di++ {
			sp := g.door[c*4+di]
			if sp[0] < 0 {
				continue
			}
			for k := sp[0]; k <= sp[1]; k++ {
				switch connDirs[di].e {
				case EN:
					g.open(ox+k, oz)
				case EE:
					g.open(ox+RoomSize-1, oz+k)
				case ES:
					g.open(ox+k, oz+RoomSize-1)
				case EW:
					g.open(ox, oz+k)
				}
			}
		}
	}
}

func (g *gen) open(x, z int) {
	if x >= 0 && x < g.w && z >= 0 && z < g.d {
		g.walls[x+z*g.w] = false
	}
}

// routeRooms returns the BFS room path spawn->elevator (§6 step 3 ordering).
func (g *gen) routeRooms(s, t int) []int {
	n := g.gw * g.gz
	prev := make([]int, n)
	for i := range prev {
		prev[i] = -2
	}
	prev[s] = -1
	q := []int{s}
	for len(q) > 0 {
		c := q[0]
		q = q[1:]
		if c == t {
			break
		}
		for _, nb := range g.neighbours(c) {
			if prev[nb] != -2 {
				continue
			}
			prev[nb] = c
			q = append(q, nb)
		}
	}
	var route []int
	for c := t; c != -1; c = prev[c] {
		route = append([]int{c}, route...)
	}
	return route
}

// pickButtonRoom prefers a room on a side branch off the main route so the
// objective is a detour, not on the straight sprint (§6 step 4, rings).
func (g *gen) pickButtonRoom(route []int, s, t int) int {
	inroute := make(map[int]bool, len(route))
	for _, c := range route {
		inroute[c] = true
	}
	var branch, fallback []int
	for c := 0; c < g.gw*g.gz; c++ {
		if !g.used[c] || c == s || c == t || g.edge[c].Count() == 4 || !g.openRoom(c) {
			continue // need an open chamber with a free wall to back the plate onto
		}
		if inroute[c] {
			fallback = append(fallback, c)
			continue
		}
		off := true
		for _, nb := range g.neighbours(c) {
			if inroute[nb] {
				off = false
			}
		}
		if !off {
			branch = append(branch, c)
		} else {
			fallback = append(fallback, c)
		}
	}
	pool := branch
	if len(pool) == 0 {
		pool = fallback
	}
	if len(pool) == 0 {
		return route[len(route)/2]
	}
	return pool[g.rng.Intn(len(pool))]
}

// roomDist returns BFS room-graph distances from src over open edges (-1 for
// unreachable or unused rooms).
func (g *gen) roomDist(src int) []int {
	dist := make([]int, g.gw*g.gz)
	for i := range dist {
		dist[i] = -1
	}
	dist[src] = 0
	q := []int{src}
	for len(q) > 0 {
		c := q[0]
		q = q[1:]
		for _, nb := range g.neighbours(c) {
			if dist[nb] != -1 {
				continue
			}
			dist[nb] = dist[c] + 1
			q = append(q, nb)
		}
	}
	return dist
}

// pickElevatorRoom chooses the exit room: an open chamber set well back from the
// button so the post-press trek is long, drawn at random from a "far" band rather
// than pinned to the single furthest room (or one fixed map corner), and never
// adjacent to the spawn. Falls back to any far room, then the deep corner, if no
// open chamber qualifies.
func (g *gen) pickElevatorRoom(spawn, button int) int {
	dB := g.roomDist(button)
	dS := g.roomDist(spawn)
	eligible := func(c int, openOnly bool) bool {
		if !g.used[c] || c == spawn || c == button || dB[c] < 0 || g.edge[c].Count() == 4 {
			return false // need a reachable room with a free wall for the cage backing
		}
		return !openOnly || g.openRoom(c)
	}
	pick := func(openOnly bool) (int, bool) {
		maxD := -1
		for c := 0; c < g.gw*g.gz; c++ {
			if eligible(c, openOnly) && dB[c] > maxD {
				maxD = dB[c]
			}
		}
		if maxD < 0 {
			return 0, false
		}
		thresh := (maxD*3 + 4) / 5 // ~0.6*maxD: the "quite far" band, random within it
		var band, farthest []int
		for c := 0; c < g.gw*g.gz; c++ {
			if !eligible(c, openOnly) {
				continue
			}
			if dB[c] == maxD {
				farthest = append(farthest, c)
			}
			if dB[c] >= thresh && (dS[c] < 0 || dS[c] >= 2) {
				band = append(band, c)
			}
		}
		if len(band) == 0 { // band emptied by the spawn-distance floor: take the farthest
			band = farthest
		}
		return band[g.rng.Intn(len(band))], true
	}
	if c, ok := pick(true); ok { // prefer an open chamber (fits the 2x2 platform, minimal carve)
		return c
	}
	if c, ok := pick(false); ok {
		return c
	}
	return g.idx(g.gw-1, g.gz-1) // safety net
}

// mountSide is a doorless wall: e is its edge bit, (dx,dz) points into the room.
type mountSide struct {
	e      Edge
	dx, dz int
}

// freeSides lists room c's doorless walls (a solid border the plate/cage backs
// onto).
func (g *gen) freeSides(c int) []mountSide {
	var free []mountSide
	for _, s := range []mountSide{{EN, 0, 1}, {ES, 0, -1}, {EW, 1, 0}, {EE, -1, 0}} {
		if g.edge[c]&s.e == 0 {
			free = append(free, s)
		}
	}
	return free
}

// pickSide returns a random doorless wall (north if the room is fully doored).
func (g *gen) pickSide(c int) mountSide {
	sides := g.freeSides(c)
	if len(sides) == 0 {
		return mountSide{EN, 0, 1}
	}
	return sides[g.rng.Intn(len(sides))]
}

// mountCell returns room c's border cell on side s at lateral position p (local
// 0..15 along the wall).
func (g *gen) mountCell(c int, s mountSide, p int) (int, int) {
	ox, oz := RoomSize*(c%g.gw), RoomSize*(c/g.gw)
	switch s.e {
	case EN:
		return ox + p, oz
	case ES:
		return ox + p, oz + RoomSize - 1
	case EW:
		return ox, oz + p
	default: // EE
		return ox + RoomSize - 1, oz + p
	}
}

// cornerBiasedPos picks a wall position (local 2..13) skewed toward the two ends,
// so the elevator tucks into a room corner more often than mid-wall.
func (g *gen) cornerBiasedPos() int {
	off := 2 + g.rng.Intn(3) // 2..4 cells in from an end
	if g.rng.Intn(2) == 0 {
		return off
	}
	return RoomSize - 1 - off // 11..13
}

// injectButton mounts the power button on a random doorless wall of room c at a
// random position along it, carving only the little niche needed to reach open
// floor (§6 step 9, anchor injection). Pos is the plate center, facing the room.
func (g *gen) injectButton(c int) (pos, facing mgl64.Vec3) {
	s := g.pickSide(c)
	p := 2 + g.rng.Intn(RoomSize-4) // 2..13: clear of the perpendicular corners
	wx, wz := g.mountCell(c, s, p)
	g.carveMount(c, wx, wz, s.dx, s.dz, 0)
	facing = mgl64.Vec3{float64(s.dx), 0, float64(s.dz)}
	wc := mgl64.Vec3{float64(wx) + 0.5, spawnEyeY, float64(wz) + 0.5}
	return wc.Add(facing.Mul(0.5 + plateThk)), facing
}

// injectElevator backs the exit cage against a doorless wall of room c, biased
// toward a corner, carving the shallow pocket its 2x2 platform needs. Pos is the
// platform floor center, Fwd points into the room.
func (g *gen) injectElevator(c int) (pos, fwd mgl64.Vec3) {
	s := g.pickSide(c)
	wx, wz := g.mountCell(c, s, g.cornerBiasedPos())
	g.carveMount(c, wx, wz, s.dx, s.dz, 1)
	fwd = mgl64.Vec3{float64(s.dx), 0, float64(s.dz)}
	wc := mgl64.Vec3{float64(wx) + 0.5, 0, float64(wz) + 0.5}
	return wc.Add(fwd.Mul(0.5 + elevHalf)), fwd
}

// carveMount opens the minimal niche for a wall-mounted structure: starting one
// cell in from the border wall (wx,wz) it advances along (dx,dz), opening a strip
// 2*halfW+1 wide, and stops the instant the strip meets the room's existing open
// interior. Open chambers connect at the first step (a shallow pocket, no
// channel); a tight room digs on to the always-open room centre as a fallback so
// the mount is never stranded. The border wall cell itself stays solid (backing).
func (g *gen) carveMount(c, wx, wz, dx, dz, halfW int) {
	lx, lz := dz, dx // lateral axis, perpendicular to the inward step
	cx, cz := RoomSize*(c%g.gw)+8, RoomSize*(c/g.gw)+8
	for depth := 1; depth < RoomSize-1; depth++ {
		x, z := wx+dx*depth, wz+dz*depth
		touched := false
		for w := -halfW; w <= halfW; w++ {
			if !g.solidAt(x+lx*w, z+lz*w) {
				touched = true
			}
			g.open(x+lx*w, z+lz*w)
		}
		if touched {
			return
		}
		if (dz != 0 && z == cz) || (dx != 0 && x == cx) {
			g.openRect(min(x, cx), min(z, cz), max(x, cx), max(z, cz))
			return
		}
	}
}

func (g *gen) openRect(x0, z0, x1, z1 int) {
	if x0 > x1 {
		x0, x1 = x1, x0
	}
	if z0 > z1 {
		z0, z1 = z1, z0
	}
	for z := z0; z <= z1; z++ {
		for x := x0; x <= x1; x++ {
			g.open(x, z)
		}
	}
}

// decorKind is one phase-B interior pattern stamped onto an open chamber.
type decorKind uint8

const (
	decorNone    decorKind = iota
	decorCorners           // four corner pillars (cover near the walls)
	decorCenter            // one big central pillar to hide behind
	decorCross             // a solid + in the middle: an ambush blind
	decorDivider           // a thin partial wall splitting most of the room
)

// decorate is the deform pass (phase B): it stamps a small budget (0..2) of
// special patterns onto open chambers to give layouts identity and create hide
// spots, while keeping them rare so they read as landmarks rather than wallpaper.
// Patterns sit in the interior (cells 3..12) so the open perimeter ring keeps
// every door connected; the Spinner's path BFS routes around them and entity
// placements nudge off any blocked centre.
func (g *gen) decorate(spawn, button, elev int) {
	g.decor = make([]decorKind, g.gw*g.gz)
	var pool []int
	for c := 0; c < g.gw*g.gz; c++ {
		if !g.used[c] || c == spawn || c == button || c == elev || !g.openRoom(c) {
			continue
		}
		pool = append(pool, c)
	}
	g.rng.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
	kinds := []decorKind{decorCorners, decorCenter, decorCross, decorDivider}
	budget := 1 + g.rng.Intn(2) // 1..2 (ineligible rooms can drop it toward 0)
	for i := 0; i < budget && i < len(pool); i++ {
		c := pool[i]
		k := kinds[g.rng.Intn(len(kinds))]
		g.decor[c] = k
		g.stampDecor(c, k)
	}
}

// stampDecor solidifies one pattern's cells inside room c (local coords 3..12).
func (g *gen) stampDecor(c int, k decorKind) {
	ox, oz := RoomSize*(c%g.gw), RoomSize*(c/g.gw)
	block := func(lx, lz int) {
		if x, z := ox+lx, oz+lz; x >= 0 && x < g.w && z >= 0 && z < g.d {
			g.walls[x+z*g.w] = true
		}
	}
	rect := func(x0, z0, x1, z1 int) {
		for z := z0; z <= z1; z++ {
			for x := x0; x <= x1; x++ {
				block(x, z)
			}
		}
	}
	switch k {
	case decorCorners:
		for _, p := range [][2]int{{3, 3}, {12, 3}, {3, 12}, {12, 12}} {
			block(p[0], p[1])
			block(p[0], p[1]+sign(7-p[1]))
		}
	case decorCenter:
		rect(6, 6, 9, 9) // one big central pillar
	case decorCross:
		rect(7, 4, 8, 11) // vertical bar
		rect(4, 7, 11, 8) // horizontal bar
	case decorDivider:
		if g.rng.Intn(2) == 0 {
			rect(7, 3, 8, 12) // vertical partition, gaps top and bottom
		} else {
			rect(3, 7, 12, 8) // horizontal partition, gaps both sides
		}
	}
}

func sign(v int) int {
	if v < 0 {
		return -1
	}
	return 1
}

// nearestOpen returns the closest walkable cell to (cx,cz) by expanding rings, so
// placements and patrol waypoints never land inside a decoration block.
func (g *gen) nearestOpen(cx, cz int) (int, int) {
	for r := 0; r < RoomSize; r++ {
		for dz := -r; dz <= r; dz++ {
			for dx := -r; dx <= r; dx++ {
				if abs(dx) != r && abs(dz) != r {
					continue // ring perimeter only
				}
				x, z := cx+dx, cz+dz
				if x >= 0 && x < g.w && z >= 0 && z < g.d && !g.walls[x+z*g.w] {
					return x, z
				}
			}
		}
	}
	return cx, cz
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// placePos is the world spawn point for a silhouette in room c: the room centre,
// nudged to the nearest walkable cell so a central pattern hosts a hider, not a
// clip.
func (g *gen) placePos(c int) mgl64.Vec3 {
	x, z := g.nearestOpen(RoomSize*(c%g.gw)+8, RoomSize*(c/g.gw)+8)
	return mgl64.Vec3{float64(x) + 0.5, 0, float64(z) + 0.5}
}

// breakSightlines drops internal walls (each with a 2-wide doorway) across any
// open stretch longer than maxRun, until no 3-wide sightline is longer. It is the
// second half of the deform pass: it caps how far the player can see and gives
// chambers dividing walls. A wall that would strand a room is reverted.
func (g *gen) breakSightlines(maxRun int) {
	for iter := 0; iter < 64; iter++ {
		axis, line, lo, hi := g.longestBand()
		if hi-lo+1 <= maxRun {
			return
		}
		// Sweep cut positions outward from the band centre and take the first that
		// holds without stranding a room; only give up (to avoid spinning on the
		// same band) once every position has failed.
		mid, cut := (lo+hi)/2, false
		for off := 0; off <= (hi-lo)/2 && !cut; off++ {
			for _, c := range [2]int{mid + off, mid - off} {
				if c >= lo && c <= hi && g.partition(axis, line, c) {
					cut = true
					break
				}
			}
		}
		if !cut {
			return // genuinely uncuttable; leave it rather than loop
		}
	}
}

func (g *gen) solidAt(x, z int) bool {
	return x < 0 || x >= g.w || z < 0 || z >= g.d || g.walls[x+z*g.w]
}

// unreachableRooms floods open cells from the spawn room centre and counts used
// rooms whose interior the flood never reaches (0 = fully connected). Used to
// validate deform-pass walls and by the quality tests.
func (g *gen) unreachableRooms() int {
	sx, sz := 8, 8
	if g.solidAt(sx, sz) {
		return -1
	}
	vis := make([]bool, g.w*g.d)
	vis[sx+sz*g.w] = true
	q := []int{sx + sz*g.w}
	for len(q) > 0 {
		c := q[0]
		q = q[1:]
		cx, cz := c%g.w, c/g.w
		for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			nx, nz := cx+d[0], cz+d[1]
			if nx < 0 || nx >= g.w || nz < 0 || nz >= g.d || g.walls[nx+nz*g.w] || vis[nx+nz*g.w] {
				continue
			}
			vis[nx+nz*g.w] = true
			q = append(q, nx+nz*g.w)
		}
	}
	bad := 0
	for c := 0; c < g.gw*g.gz; c++ {
		if !g.used[c] {
			continue
		}
		ox, oz := RoomSize*(c%g.gw), RoomSize*(c/g.gw)
		any := false
		for z := 1; z < RoomSize-1 && !any; z++ {
			for x := 1; x < RoomSize-1; x++ {
				if !g.walls[(ox+x)+(oz+z)*g.w] && vis[(ox+x)+(oz+z)*g.w] {
					any = true
					break
				}
			}
		}
		if !any {
			bad++
		}
	}
	return bad
}

// longestBand finds the longest run of cells whose 3-wide perpendicular band is
// open (the sightline measure), returning its axis (0=along a row, 1=along a
// column), the fixed line, and the run's [lo,hi] span.
func (g *gen) longestBand() (axis, line, lo, hi int) {
	best := 0
	take := func(ax, ln, end, run int) {
		if run > best {
			best, axis, line, lo, hi = run, ax, ln, end-run, end-1
		}
	}
	for z := 0; z < g.d; z++ {
		run := 0
		for x := 0; x < g.w; x++ {
			if !g.solidAt(x, z) && !g.solidAt(x, z-1) && !g.solidAt(x, z+1) {
				run++
			} else {
				take(0, z, x, run)
				run = 0
			}
		}
		take(0, z, g.w, run)
	}
	for x := 0; x < g.w; x++ {
		run := 0
		for z := 0; z < g.d; z++ {
			if !g.solidAt(x, z) && !g.solidAt(x-1, z) && !g.solidAt(x+1, z) {
				run++
			} else {
				take(1, x, z, run)
				run = 0
			}
		}
		take(1, x, g.d, run)
	}
	return
}

// partition raises a wall perpendicular to a sightline at (axis,line) crossing
// position cut, spanning the open strip there (clamped to room scale) with a
// 2-wide doorway. Returns false (after reverting) if it would strand a room.
func (g *gen) partition(axis, line, cut int) bool {
	var cells [][2]int
	if axis == 0 { // run along a row -> wall is the vertical column x=cut
		x, z0, z1 := cut, line, line
		for z0 > 0 && !g.solidAt(x, z0-1) && line-z0 < 8 {
			z0--
		}
		for z1 < g.d-1 && !g.solidAt(x, z1+1) && z1-line < 8 {
			z1++
		}
		for z := z0; z <= z1; z++ {
			cells = append(cells, [2]int{x, z})
		}
	} else { // run along a column -> wall is the horizontal row z=cut
		z, x0, x1 := cut, line, line
		for x0 > 0 && !g.solidAt(x0-1, z) && line-x0 < 8 {
			x0--
		}
		for x1 < g.w-1 && !g.solidAt(x1+1, z) && x1-line < 8 {
			x1++
		}
		for x := x0; x <= x1; x++ {
			cells = append(cells, [2]int{x, z})
		}
	}
	if len(cells) < 3 {
		return false // too short to host a wall + doorway
	}
	for _, c := range cells {
		g.walls[c[0]+c[1]*g.w] = true
	}
	gap := 1 + g.rng.Intn(len(cells)-2) // interior doorway: wall remains both ends
	g.walls[cells[gap][0]+cells[gap][1]*g.w] = false
	g.walls[cells[gap-1][0]+cells[gap-1][1]*g.w] = false
	if g.unreachableRooms() != 0 {
		for _, c := range cells {
			g.walls[c[0]+c[1]*g.w] = false
		}
		return false
	}
	return true
}

// spawnFacingYaw returns the camera yaw that faces the first open doorway in
// the spawn room. Priority: East, South, West, North (the spawn room corner
// can only have East/South doors, but the fallback covers braided layouts).
func spawnFacingYaw(e Edge) float64 {
	dirs := [4]struct {
		bit    Edge
		dx, dz float64
	}{
		{EE, 1, 0},
		{ES, 0, 1},
		{EW, -1, 0},
		{EN, 0, -1},
	}
	for _, d := range dirs {
		if e&d.bit != 0 {
			return math.Atan2(d.dx, d.dz)
		}
	}
	return 0
}

// valid flood-fills from spawn and asserts the button and elevator stand cells
// are reachable (§6 step 8).
func (g *gen) valid(m *Manifest) bool {
	sx, sz := int(math.Floor(m.Spawn.X())), int(math.Floor(m.Spawn.Z()))
	if sx < 0 || sx >= g.w || g.walls[sx+sz*g.w] {
		return false
	}
	vis := make([]bool, g.w*g.d)
	q := []int{sx + sz*g.w}
	vis[q[0]] = true
	for len(q) > 0 {
		c := q[0]
		q = q[1:]
		cx, cz := c%g.w, c/g.w
		for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			nx, nz := cx+d[0], cz+d[1]
			if nx < 0 || nx >= g.w || nz < 0 || nz >= g.d || g.walls[nx+nz*g.w] {
				continue
			}
			ni := nx + nz*g.w
			if vis[ni] {
				continue
			}
			vis[ni] = true
			q = append(q, ni)
		}
	}
	reach := func(p, dir mgl64.Vec3) bool {
		x := int(math.Floor(p.X() + dir.X()))
		z := int(math.Floor(p.Z() + dir.Z()))
		return x >= 0 && x < g.w && z >= 0 && z < g.d && vis[x+z*g.w]
	}
	return reach(m.ButtonPos, m.ButtonFacing) && reach(m.ElevatorPos, mgl64.Vec3{})
}
