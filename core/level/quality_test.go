package level

import (
	"fmt"
	"math/rand"
	"testing"
)

// TestLayoutQuality locks in the generator's space-usage goals across a wide seed
// sweep: every used room is reachable (no wasted/inaccessible rooms), most of the
// map is walkable (no vast solid voids), and dead ends stay rare. It dumps the
// offending layout as ASCII on failure.
func TestLayoutQuality(t *testing.T) {
	lib := testLib()
	const maxBand = 32 // longest see-down-a-corridor run we tolerate (~2 rooms)
	var sumOpen float64
	var sumDead, sumBand, worstBand, n int
	for seed := int64(0); seed < 300; seed++ {
		for diff := 0; diff < 4; diff++ {
			g := newGen(rand.New(rand.NewSource(seed)), diff, lib)
			lvl, _ := g.run(diff)
			if bad := g.unreachableRooms(); bad != 0 {
				t.Fatalf("seed %d diff %d: %d unreachable rooms\n%s", seed, diff, bad, ascii(lvl))
			}
			open := openFrac(lvl)
			if open < 0.40 {
				t.Fatalf("seed %d diff %d: only %.0f%% walkable\n%s", seed, diff, 100*open, ascii(lvl))
			}
			dead, _ := g.deadEnds()
			if dead > 2 {
				t.Fatalf("seed %d diff %d: %d dead ends\n%s", seed, diff, dead, ascii(lvl))
			}
			if dc := decorCount(g); dc > 2 {
				t.Fatalf("seed %d diff %d: %d decorations (cap 2)\n%s", seed, diff, dc, ascii(lvl))
			}
			band := maxOpenBand(lvl)
			if band > maxBand {
				t.Fatalf("seed %d diff %d: open sightline run %d > %d\n%s", seed, diff, band, maxBand, ascii(lvl))
			}
			sumOpen += open
			sumDead += dead
			sumBand += band
			if band > worstBand {
				worstBand = band
			}
			n++
		}
	}
	if avg := sumOpen / float64(n); avg < 0.48 {
		t.Fatalf("avg walkable %.1f%% below target", 100*avg)
	} else {
		t.Logf("avg walkable %.1f%%  dead ends %d  sightline avg %d worst %d  over %d levels",
			100*avg, sumDead, sumBand/n, worstBand, n)
	}
}

// decorCount returns how many rooms the deform pass patterned.
func decorCount(g *gen) int {
	n := 0
	for _, d := range g.decor {
		if d != decorNone {
			n++
		}
	}
	return n
}

// maxOpenBand is the longest straight stretch a player can actually see down: the
// longest run along any row (or column) whose 3-wide perpendicular band is fully
// open. A thin slit flanked by walls does not count, so this tracks genuine
// wide-open sightlines rather than 1-cell gaps between cover.
func maxOpenBand(l *Level) int {
	best := 0
	consider := func(run int) {
		if run > best {
			best = run
		}
	}
	for z := 0; z < l.depth; z++ {
		run := 0
		for x := 0; x < l.width; x++ {
			if !l.Solid(x, z) && !l.Solid(x, z-1) && !l.Solid(x, z+1) {
				run++
				consider(run)
			} else {
				run = 0
			}
		}
	}
	for x := 0; x < l.width; x++ {
		run := 0
		for z := 0; z < l.depth; z++ {
			if !l.Solid(x, z) && !l.Solid(x-1, z) && !l.Solid(x+1, z) {
				run++
				consider(run)
			} else {
				run = 0
			}
		}
	}
	return best
}

// openFrac is the share of cells that are walkable floor.
func openFrac(l *Level) float64 {
	open, cells := 0, l.width*l.depth
	for z := 0; z < l.depth; z++ {
		for x := 0; x < l.width; x++ {
			if !l.Solid(x, z) {
				open++
			}
		}
	}
	return float64(open) / float64(cells)
}

// deadEnds counts used rooms with a single door.
func (g *gen) deadEnds() (dead, rooms int) {
	for c := 0; c < g.gw*g.gz; c++ {
		if !g.used[c] {
			continue
		}
		rooms++
		if g.edge[c].Count() == 1 {
			dead++
		}
	}
	return
}

// ascii renders a level as a wall map for failure diagnostics.
func ascii(l *Level) string {
	out := make([]byte, 0, (l.width+1)*l.depth)
	for z := 0; z < l.depth; z++ {
		for x := 0; x < l.width; x++ {
			if l.Solid(x, z) {
				out = append(out, '#')
			} else {
				out = append(out, ' ')
			}
		}
		out = append(out, '\n')
	}
	return fmt.Sprintf("%dx%d\n%s", l.width, l.depth, out)
}
