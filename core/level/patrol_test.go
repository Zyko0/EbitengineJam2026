package level

import (
	"math/rand"
	"testing"
)

// TestCircuitCoverage checks two properties for each generated level individually
// (not just on average across N runs):
//
//   - Both circuits together cover >= 90% of all rooms (union).
//   - The two circuits share <= 10% of rooms in common (intersection).
func TestCircuitCoverage(t *testing.T) {
	const (
		N          = 50
		baseSeed   = 1337
		minCover   = 0.90
		maxOverlap = 0.10
	)
	lib := testLib()
	for i := range N {
		seed := int64(baseSeed + i)
		g := newGen(rand.New(rand.NewSource(seed)), 1, lib)
		lvl, _ := g.run(1)

		totalRooms := 0
		for c := range g.gw * g.gz {
			if g.used[c] {
				totalRooms++
			}
		}
		if totalRooms == 0 {
			t.Fatalf("seed %d: no rooms generated", seed)
		}

		_, _, wallCycle, directCycle := g.buildCircuits(lvl)

		if len(wallCycle) == 0 {
			t.Errorf("seed %d: wall circuit is empty", seed)
		}
		if len(directCycle) == 0 {
			t.Errorf("seed %d: direct circuit is empty", seed)
		}

		wallSet := make(map[int]bool, len(wallCycle))
		for _, r := range wallCycle {
			wallSet[r] = true
		}
		directSet := make(map[int]bool, len(directCycle))
		for _, r := range directCycle {
			directSet[r] = true
		}

		// Union coverage: fraction of all rooms visited by at least one circuit.
		union := make(map[int]bool, len(wallSet)+len(directSet))
		for r := range wallSet {
			union[r] = true
		}
		for r := range directSet {
			union[r] = true
		}
		coverage := float64(len(union)) / float64(totalRooms)
		if coverage < minCover {
			t.Errorf("seed %d: coverage %.0f%% < %.0f%% (wall=%d direct=%d total=%d)",
				seed, 100*coverage, 100*minCover, len(wallSet), len(directSet), totalRooms)
		}

		// Intersection overlap: fraction of rooms in both circuits.
		intersect := 0
		for r := range wallSet {
			if directSet[r] {
				intersect++
			}
		}
		overlap := float64(intersect) / float64(totalRooms)
		if overlap > maxOverlap {
			t.Errorf("seed %d: overlap %.0f%% > %.0f%% (%d rooms shared)",
				seed, 100*overlap, 100*maxOverlap, intersect)
		}
	}
}
