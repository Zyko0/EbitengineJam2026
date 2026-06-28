package level

import (
	"math"
	"testing"
)

// testLib mirrors assets.Rooms (plain shapes: corridors, bends, corridor T/cross
// intersections, rare open halls, specials) so generation can be validated
// without importing the cgo-heavy assets package. Keep it in sync with that file.
func testLib() []Room {
	open := func(s *RoomStamp) { s.OpenRect(1, 1, 14, 14) }
	return []Room{
		{Name: "corr_wide", Edges: EN | ES, Tags: TagCorridor | TagLongLane, Weight: 3,
			Carve: func(s *RoomStamp) { s.OpenRect(5, 0, 10, 15) }},
		{Name: "corr_choke", Edges: EN | ES, Tags: TagCorridor | TagChokepoint, Weight: 2,
			Carve: func(s *RoomStamp) {
				s.OpenRect(5, 0, 10, 15)
				for z := 6; z <= 9; z++ {
					s.Block(5, z)
					s.Block(6, z)
					s.Block(9, z)
					s.Block(10, z)
				}
			}},
		{Name: "hall_open2", Edges: EN | ES, Tags: TagHall, Weight: 3, Carve: open},
		{Name: "bend_wide", Edges: EN | EE, Tags: TagCorridor, Weight: 4,
			Carve: func(s *RoomStamp) { s.OpenRect(5, 0, 10, 10); s.OpenRect(5, 5, 15, 10) }},
		{Name: "room_corner", Edges: EN | EE, Tags: TagHall, Weight: 2, Carve: open},
		{Name: "junc_T", Edges: EN | EE | ES, Tags: TagCorridor | TagJunction, Weight: 4,
			Carve: func(s *RoomStamp) { s.OpenRect(5, 0, 10, 15); s.OpenRect(5, 5, 15, 10) }},
		{Name: "hall_open3", Edges: EN | EE | ES, Tags: TagHall | TagJunction, Weight: 2, Carve: open},
		{Name: "junc_cross", Edges: EN | EE | ES | EW, Tags: TagCorridor | TagJunction, Weight: 4,
			Carve: func(s *RoomStamp) { s.OpenRect(5, 0, 10, 15); s.OpenRect(0, 5, 15, 10) }},
		{Name: "hall_open4", Edges: EN | EE | ES | EW, Tags: TagHall | TagJunction, Weight: 2, Carve: open},
		{Name: "room_end", Edges: EN, Tags: TagDeadEnd, Weight: 3, Carve: open},
		{Name: "room_spawn", Edges: EN, Tags: TagSpawn | TagDeadEnd, Weight: 1,
			Carve: func(s *RoomStamp) { s.OpenRect(1, 1, 14, 14) }},
		{Name: "room_elevator", Edges: EN, Tags: TagElevator, Weight: 1,
			Carve: func(s *RoomStamp) { s.OpenRect(1, 1, 14, 14) }},
	}
}

func TestGenerateSolvable(t *testing.T) {
	lib := testLib()
	for seed := int64(0); seed < 200; seed++ {
		for diff := 0; diff < 4; diff++ {
			lvl, m := Generate(seed, diff, lib)
			if lvl == nil || m == nil {
				t.Fatalf("seed %d diff %d: nil result", seed, diff)
			}
			// Budget fits the shader uniform.
			if got := (lvl.Width() + 31) / 32 * lvl.Depth(); got > MaxWallInts {
				t.Fatalf("seed %d: %d ints exceeds budget %d", seed, got, MaxWallInts)
			}
			// Spawn, button stand and elevator stand all mutually reachable.
			sx, sz := int(math.Floor(m.Spawn.X())), int(math.Floor(m.Spawn.Z()))
			bx := int(math.Floor(m.ButtonPos.X() + m.ButtonFacing.X()))
			bz := int(math.Floor(m.ButtonPos.Z() + m.ButtonFacing.Z()))
			ex, ez := int(math.Floor(m.ElevatorPos.X())), int(math.Floor(m.ElevatorPos.Z()))
			if lvl.BFSPath(sx, sz, bx, bz) == nil {
				t.Fatalf("seed %d diff %d: button unreachable from spawn", seed, diff)
			}
			if lvl.BFSPath(sx, sz, ex, ez) == nil {
				t.Fatalf("seed %d diff %d: elevator unreachable from spawn", seed, diff)
			}
		}
	}
}

func TestPatrolCircuitClosed(t *testing.T) {
	lib := testLib()
	loops := 0
	for seed := int64(0); seed < 100; seed++ {
		lvl, _ := Generate(seed, 1, lib)
		c := lvl.PatrolCircuit()
		if len(c) == 0 {
			continue
		}
		loops++
		// Consecutive waypoints (including the wrap) must share a row or column:
		// the Spinner only runs straight axis-aligned lanes.
		for i := range c {
			a, b := c[i], c[(i+1)%len(c)]
			if a.X() != b.X() && a.Z() != b.Z() {
				t.Fatalf("seed %d: diagonal patrol leg %v->%v", seed, a, b)
			}
		}
	}
	if loops == 0 {
		t.Fatal("no level produced a patrol loop")
	}
}

func TestEdgeRotate(t *testing.T) {
	if got := (EN | EE).Rotate(1); got != EE|ES {
		t.Fatalf("rotate EN|EE by 1 = %04b, want %04b", got, EE|ES)
	}
	if got := EN.Rotate(4); got != EN {
		t.Fatalf("rotate by 4 should be identity, got %04b", got)
	}
}
