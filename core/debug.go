package core

import (
	"image/color"
	"time"

	"github.com/Zyko0/EbitengineJam2026/assets"
	xaudio "github.com/Zyko0/EbitengineJam2026/audio"
	"github.com/Zyko0/EbitengineJam2026/core/entity"
	"github.com/Zyko0/EbitengineJam2026/core/level"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// Self-contained debug overlay: a top-down minimap in the lower-right corner and
// an R hotkey to roll a fresh level. Remove this file to drop the whole feature.

const (
	minimapFrac     = 3  // a reference square level fills ≈ screen height / minimapFrac
	minimapRefCells = 64 // cell extent of that reference square level (4 rooms × RoomSize)
	minimapMargin   = 10.0
)

var (
	colFree     = color.RGBA{18, 22, 28, 220}
	colWall     = color.RGBA{90, 100, 120, 255}
	colPlayer   = color.RGBA{70, 200, 240, 255}
	colButton   = color.RGBA{240, 220, 60, 255}
	colButtonHi = color.RGBA{110, 100, 30, 255} // already pressed
	colElevator = color.RGBA{60, 230, 90, 255}
	colEnemy    = color.RGBA{230, 60, 60, 255}
	colSpinner  = color.RGBA{240, 140, 40, 255}
)

// updateDebug handles debug-only input. R rolls a brand new game (difficulty
// reset, fresh seed). Called from Game.Update.
func (g *Game) updateDebug() {
	if inpututil.IsKeyJustPressed(ebiten.KeyR) {
		g.restart()
	}
}

// restart resets to a freshly generated level at difficulty 0, teleporting the
// player to the new spawn (mirrors regenerate without bumping difficulty).
func (g *Game) restart() {
	g.difficulty = 0
	lvl, m := level.Generate(time.Now().UnixNano(), 0, assets.Rooms)
	g.levels = []*level.Level{lvl}
	g.levelIndex = 0
	g.player.Pos = m.Spawn
	g.player.HP = PlayerMaxHP
	g.player.Invuln = 0
	g.player.Knockback = mgl64.Vec3{}
	g.camera.SetYawPitch(m.SpawnYaw, 0)
	// Silence the old run's loops before the new entities claim the shared looping
	// players, so a restart never stacks a second copy.
	xaudio.StopLoops()
	g.entities = NewEntityManager(m)
	g.exit = ExitSequence{}
	g.catch = CatchSequence{}
	g.disconnect.Unlock()
	// Fresh run: reset the run-wide silhouette budget and clear every overlay.
	g.silhouette = entity.NewWhiteSilhouette()
	g.dead = false
	g.deathBySilhouette = false
	g.gameover.Hide()
	g.victory.Hide()
	g.pause.Hide()
	g.pause.ResetObjectives()
	g.exitOn = false
	g.announceFloor()
}

// drawMinimap renders a top-down debug view of the active level in the lower
// right corner: walls, free space, button, elevator, enemies and the player.
func (g *Game) drawMinimap(screen *ebiten.Image) {
	lvl := g.levels[g.levelIndex]
	w, d := lvl.Width(), lvl.Depth()
	if w == 0 || d == 0 {
		return
	}
	sb := screen.Bounds()
	// One fixed whole-pixel cell size, calibrated so a reference square level fills
	// ~1/minimapFrac of the screen height. Every layout uses this same cell size, so
	// long/tall levels keep the square's proportions and simply extend further along
	// their long side instead of being shrunk to fit.
	cell := float32(max(1, sb.Dy()/minimapFrac/minimapRefCells))
	mw, mh := cell*float32(w), cell*float32(d)
	ox := float32(sb.Dx()) - mw - minimapMargin
	oy := float32(sb.Dy()) - mh - minimapMargin

	vector.FillRect(screen, ox, oy, mw, mh, colFree, false)
	for z := 0; z < d; z++ {
		for x := 0; x < w; x++ {
			if lvl.Solid(x, z) {
				vector.FillRect(screen, ox+float32(x)*cell, oy+float32(z)*cell, cell, cell, colWall, false)
			}
		}
	}

	dot := func(p mgl64.Vec3, r float32, c color.Color) {
		vector.FillCircle(screen, ox+float32(p.X())*cell, oy+float32(p.Z())*cell, r, c, true)
	}

	if b := g.entities.button; b != nil {
		c := colButton
		if b.Pressed {
			c = colButtonHi
		}
		dot(b.Pos, cell*0.6, c)
	}
	if e := g.entities.elevator; e.Active {
		dot(e.Pos, cell*0.7, colElevator)
	}
	for _, e := range g.entities.entities {
		c := colEnemy
		if _, ok := e.(*entity.PatrollerWall); ok {
			c = colSpinner
		}
		dot(e.Position(), cell*0.5, c)
	}

	// Player on top, with a facing tick.
	px := ox + float32(g.player.Pos.X())*cell
	py := oy + float32(g.player.Pos.Z())*cell
	vector.FillCircle(screen, px, py, cell*0.6, colPlayer, true)
	dir := g.camera.Direction()
	vector.StrokeLine(screen, px, py, px+float32(dir.X())*cell*1.6, py+float32(dir.Z())*cell*1.6, 1.5, color.White, true)
}
