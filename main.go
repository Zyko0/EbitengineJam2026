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
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

type Game struct {
	offscreen *ebiten.Image
	game      *core.Game
	updated   bool
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
		return nil
	}
	// Escape quits on native. On wasm the browser also uses it to drop the pointer
	// lock, so there it toggles the pause menu instead (handy when the player
	// defocuses the window).
	if runtime.GOOS == "js" {
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			g.game.TogglePause()
		}
	} else if ebiten.IsKeyPressed(ebiten.KeyEscape) {
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
