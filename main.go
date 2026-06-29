package main

import (
	"log"
	"runtime"

	"github.com/Zyko0/EbitengineJam2026/assets"
	"github.com/Zyko0/EbitengineJam2026/core"
	"github.com/Zyko0/EbitengineJam2026/input"
	"github.com/Zyko0/EbitengineJam2026/logic"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/hajimehoshi/ebiten/v2"
)

type Game struct {
	offscreen   *ebiten.Image
	game        *core.Game
	updated     bool
	wasCaptured bool // wasm: tracks pointer-lock loss to pause on Escape/defocus
}

func New() *Game {
	return &Game{
		offscreen: ebiten.NewImage(logic.ScreenWidth, logic.ScreenHeight),
		game: core.NewGame(core.NewCamera(
			mgl64.Vec3{1.5, 3, 1.5},
			mgl64.Vec3{0, 0, 0},
			logic.CameraFoV,
			float64(logic.ScreenWidth)/float64(logic.ScreenHeight),
		)),
	}
}

func (g *Game) Update() error {
	if !input.EnsureCursorCaptured() {
		// On wasm the browser eats the Escape keydown to release the pointer lock,
		// so a lock we previously held disappearing is our only Escape/defocus
		// signal. Raise the pause (skip the initial pre-click state so the game
		// doesn't start paused); the run then resumes deliberately on the next click.
		if runtime.GOOS == "js" && g.wasCaptured {
			g.game.Pause()
		}
		g.wasCaptured = false
		return nil
	}
	g.wasCaptured = true
	// Escape quits on native; on wasm it releases the pointer lock, handled above.
	if runtime.GOOS != "js" && ebiten.IsKeyPressed(ebiten.KeyEscape) {
		return ebiten.Termination
	}

	g.game.Update()
	g.updated = false

	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	if !g.updated {
		g.game.Draw(g.offscreen)
		g.updated = true
	}

	screen.DrawImage(g.offscreen, nil)

	/*p := g.game.PlayerPosition()
	ebitenutil.DebugPrint(screen, fmt.Sprintf(
		"FPS: %.2f - Pos: %.2f, %.2f, %.2f - Charge: %.2f",
		ebiten.ActualFPS(), p[0], p[1], p[2], g.game.Disconnect().Charge,
	))*/
}

func (g *Game) DrawFinalScreen(screen ebiten.FinalScreen, offscreen *ebiten.Image, geoM ebiten.GeoM) {
	w, h := offscreen.Bounds().Dx(), offscreen.Bounds().Dy()
	screen.DrawRectShader(w, h, assets.ShaderPostProcess(), &ebiten.DrawRectShaderOptions{
		GeoM:   geoM,
		Images: [4]*ebiten.Image{offscreen},
		Uniforms: map[string]any{
			"Time": float32(g.game.Ticks()) / logic.TPS,
		},
	})
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return logic.ScreenWidth, logic.ScreenHeight
}

func main() {
	ebiten.SetVsyncEnabled(true)
	ebiten.SetTPS(logic.TPS)
	ebiten.SetFullscreen(true)
	ebiten.SetWindowSize(logic.ScreenWidth, logic.ScreenHeight)
	ebiten.SetCursorShape(ebiten.CursorShapeCrosshair)

	if err := ebiten.RunGameWithOptions(New(), &ebiten.RunGameOptions{}); err != nil {
		log.Fatal("rungame:", err)
	}
}
