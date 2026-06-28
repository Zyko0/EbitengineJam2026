package level

// RoomSize is the cell extent of one authored building block.
const RoomSize = 16

// Door slot: 6-wide, centered on the edge (local cells doorLo..doorHi), so two
// abutting open edges always line up and adjacent rooms read as joined, not
// pinholed. Every template's lane spans at least these cells per door.
const doorLo, doorHi = 5, 10

// Edge is a bitmask of which sides carry a door. N points toward -Z (smaller z),
// rotating clockwise N->E->S->W.
type Edge uint8

const (
	EN Edge = 1 << iota // north, -Z
	EE                  // east,  +X
	ES                  // south, +Z
	EW                  // west,  -X
)

// Rotate turns the edge mask clockwise by r quarter-turns (matches RoomStamp).
func (e Edge) Rotate(r int) Edge {
	r &= 3
	v := uint8(e)

	return Edge((v<<r | v>>(4-r)) & 0xF)
}

// Count returns how many doors the mask carries.
func (e Edge) Count() int {
	n := 0
	for v := uint8(e); v != 0; v &= v - 1 {
		n++
	}

	return n
}

type Tag uint16

const (
	TagCorridor Tag = 1 << iota
	TagJunction
	TagHall       // large open interior
	TagPillars    // pillar cover (breaks LoS)
	TagAlcove     // side niche(s)
	TagChokepoint // narrow single-lane pinch
	TagLongLane   // a clear straight run end-to-end
	TagDeadEnd    // exactly one open edge
	TagSpawn      // player start
	TagButton     // can mount the power button
	TagElevator   // can host the exit cage
)

func (t Tag) Has(o Tag) bool {
	return t&o != 0
}

// Room is an authored template: a shape (Carve) plus the metadata that gates
// decoration and placement once the shape is fixed (§3).
type Room struct {
	Name    string
	Edges   Edge // doors this template provides (base orientation)
	Tags    Tag
	Carve   func(s *RoomStamp) // opens interior into a solid 16x16 block
	Weight  int
}

// RoomStamp writes a template's interior into the map's cell grid, applying the
// room's world origin and rotation so Carve funcs author in plain local coords.
type RoomStamp struct {
	walls  []bool
	mapW   int
	ox, oz int // world origin cell of the room
	rot    int // clockwise quarter-turns
}

// rotateLocal maps a local cell (0..15) through rot clockwise quarter-turns.
func (s *RoomStamp) rotateLocal(lx, lz int) (int, int) {
	x, z := lx, lz
	for i := 0; i < (s.rot & 3); i++ {
		x, z = RoomSize-1-z, x
	}

	return x, z
}

func (s *RoomStamp) put(lx, lz int, wall bool) {
	x, z := s.rotateLocal(lx, lz)
	s.walls[(s.ox+x)+(s.oz+z)*s.mapW] = wall
}

// Open clears a local cell
func (s *RoomStamp) Open(lx, lz int) {
	s.put(lx, lz, false)
}

// Block fills a local cell
func (s *RoomStamp) Block(lx, lz int) {
	s.put(lx, lz, true)
}

func (s *RoomStamp) OpenRect(x0, z0, x1, z1 int) {
	for z := z0; z <= z1; z++ {
		for x := x0; x <= x1; x++ {
			s.Open(x, z)
		}
	}
}
