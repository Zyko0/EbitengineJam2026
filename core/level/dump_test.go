package level

import (
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"testing"
)

// TestDumpASCII renders layouts to /tmp/levels.txt for visual iteration. Set
// DUMP=seed:diff,seed:diff,... (or DUMP=1 for a default spread).
//
//	DUMP=0:2 CGO_ENABLED=0 go test ./core/level/ -run TestDumpASCII
func TestDumpASCII(t *testing.T) {
	spec := os.Getenv("DUMP")
	if spec == "" {
		t.Skip("set DUMP=seed:diff,... to dump")
	}
	lib := testLib()
	var b strings.Builder
	dump := func(seed int64, diff int) {
		g := newGen(rand.New(rand.NewSource(seed)), diff, lib)
		lvl, _ := g.run(diff)
		fmt.Fprintf(&b, "seed %d diff %d  walkable %.0f%%  sightline %d  decor %d  unreachable %d\n",
			seed, diff, 100*openFrac(lvl), maxOpenBand(lvl), decorCount(g), g.unreachableRooms())
		// flood from spawn centre; mark open-but-unreached cells with '?'.
		vis := make([]bool, g.w*g.d)
		if !g.walls[8+8*g.w] {
			vis[8+8*g.w] = true
			q := []int{8 + 8*g.w}
			for len(q) > 0 {
				c := q[0]
				q = q[1:]
				cx, cz := c%g.w, c/g.w
				for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
					nx, nz := cx+d[0], cz+d[1]
					if nx >= 0 && nx < g.w && nz >= 0 && nz < g.d && !g.walls[nx+nz*g.w] && !vis[nx+nz*g.w] {
						vis[nx+nz*g.w] = true
						q = append(q, nx+nz*g.w)
					}
				}
			}
		}
		for z := 0; z < g.d; z++ {
			for x := 0; x < g.w; x++ {
				switch {
				case g.walls[x+z*g.w]:
					b.WriteByte('#')
				case !vis[x+z*g.w]:
					b.WriteByte('?')
				default:
					b.WriteByte(' ')
				}
			}
			b.WriteByte('\n')
		}
		dirName := [4]string{"N", "E", "S", "W"}
		for c := 0; c < g.gw*g.gz; c++ {
			if !g.used[c] {
				continue
			}
			name := "open"
			if g.tmpl[c] >= 0 {
				name = g.lib[g.tmpl[c]].Name
			}
			fmt.Fprintf(&b, "  room(%d,%d) edge=%04b %-12s decor=%d doors[", c%g.gw, c/g.gw, g.edge[c], name, g.decor[c])
			for di := 0; di < 4; di++ {
				if sp := g.door[c*4+di]; sp[0] >= 0 {
					fmt.Fprintf(&b, "%s%d-%d ", dirName[di], sp[0], sp[1])
				}
			}
			b.WriteString("]\n")
		}
		b.WriteString("\n")
	}
	if spec == "1" {
		for seed := int64(0); seed < 6; seed++ {
			dump(seed, 1)
		}
	} else {
		for _, pair := range strings.Split(spec, ",") {
			sd := strings.SplitN(pair, ":", 2)
			seed, _ := strconv.ParseInt(sd[0], 10, 64)
			diff := 1
			if len(sd) == 2 {
				diff, _ = strconv.Atoi(sd[1])
			}
			dump(seed, diff)
		}
	}
	if err := os.WriteFile("/tmp/levels.txt", []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("wrote /tmp/levels.txt (%d bytes)", b.Len())
}
