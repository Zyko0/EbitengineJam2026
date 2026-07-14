package core

import (
	"fmt"
	"math"
	"time"

	"github.com/Zyko0/EbitengineJam2026/assets"
	xaudio "github.com/Zyko0/EbitengineJam2026/audio"
	"github.com/Zyko0/EbitengineJam2026/core/entity"
	"github.com/Zyko0/EbitengineJam2026/core/level"
	"github.com/Zyko0/EbitengineJam2026/graphics"
	"github.com/Zyko0/EbitengineJam2026/input"
	"github.com/Zyko0/EbitengineJam2026/logic"
	"github.com/Zyko0/EbitengineJam2026/ui"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

type Game struct {
	camera            *Camera
	player            *Player
	levels            []*level.Level
	levelIndex        int
	difficulty        int
	ticks             int
	disconnect        *DisconnectState
	entities          *EntityManager
	silhouette        *entity.WhiteSilhouette // run-wide disconnect budget; survives regenerate
	exit              ExitSequence
	catch             CatchSequence
	dead              bool
	deathBySilhouette bool // killed by the silhouette: swing the death gaze onto it

	hud      *ui.HUD
	info     *ui.InfoText
	victory  *ui.Victory
	gameover *ui.GameOver
	pause    *ui.Pause
	exitOn   bool // exit elevator was powered on last tick (info-banner edge detect)
}

func NewGame(camera *Camera) *Game {
	lvl, m := level.Generate(time.Now().UnixNano(), 0, assets.Rooms)
	camera.SetYawPitch(m.SpawnYaw, 0)

	g := &Game{
		camera:     camera,
		player:     newPlayer(m.Spawn, camera.FoV()),
		levels:     []*level.Level{lvl},
		disconnect: NewDisconnectState(),
		entities:   NewEntityManager(m),
		silhouette: entity.NewWhiteSilhouette(),
		hud:        ui.NewHUD(),
		info:       ui.NewInfoText(),
		victory:    ui.NewVictory(),
		gameover:   ui.NewGameOver(),
		pause:      ui.NewPause(),
	}
	g.announceFloor()

	return g
}

// announceFloor flashes the current floor number in the info banner.
func (g *Game) announceFloor() {
	g.info.Push(fmt.Sprintf("Floor %d", g.difficulty+1))
}

// regenerate builds the next level on elevator ascent: difficulty up, fresh
// layout and entities, player teleported to the new spawn (GAME.md §6.7).
func (g *Game) regenerate() {
	g.difficulty++
	lvl, m := level.Generate(time.Now().UnixNano(), g.difficulty, assets.Rooms)
	g.levels = []*level.Level{
		lvl,
	}
	g.levelIndex = 0
	g.player.Pos = m.Spawn
	g.camera.SetYawPitch(m.SpawnYaw, 0)
	// Silence the old floor's loops before the new entities claim the shared
	// looping players, so re-entering never stacks a second copy.
	xaudio.StopLoops()
	g.entities = NewEntityManager(m)
	g.exit = ExitSequence{}
	g.catch = CatchSequence{}
	g.disconnect.Unlock()
	g.victory.Hide()
	g.pause.Hide()
	g.pause.ResetObjectives()
	g.exitOn = false
	g.announceFloor()
}

// tickSeconds is one update tick in seconds, for the overlay fade timers.
const tickSeconds = 1.0 / logic.TPS

// maxCollisionStep caps how far the player may advance per collision sub-step.
// Kept well under a wall's one-unit thickness so no single move (e.g. a 1.5
// knockback) can sweep past a wall without overlapping it.
const maxCollisionStep = 0.4

// floorVec3 floors each component (camera voxel split / collision sweep).
func floorVec3(v mgl64.Vec3) mgl64.Vec3 {
	return mgl64.Vec3{math.Floor(v.X()), math.Floor(v.Y()), math.Floor(v.Z())}
}

// resolveWallCollision pushes the player's box at pos out of every overlapping
// solid cell via minimum-translation vectors and returns the corrected position.
// Called once per collision sub-step so fast moves resolve against walls instead
// of skipping them.
func (g *Game) resolveWallCollision(pos mgl64.Vec3) mgl64.Vec3 {
	size := mgl64.Vec3{PlayerWidth, PlayerHeight, PlayerWidth}
	ns := g.levels[g.levelIndex].AppendAround(nil, pos, size)
	box := NewAABB(pos, size)
	blockSize := mgl64.Vec3{1, 1, 1}
	for _, n := range ns {
		aabb := NewAABB(n.Add(blockSize.Mul(0.5)), blockSize)
		if !box.TestAABB(aabb) {
			continue
		}
		mtv := box.ResolveAABB_MTV(aabb)
		if mtv[0] == 0 && mtv[1] == 0 && mtv[2] == 0 {
			continue
		}
		pos = pos.Add(mtv)
		box.SetPosition(pos)
	}

	return pos
}

// Pause raises the pause overlay from outside the update loop. On wasm, main
// calls this when the browser pointer lock is lost (the player pressed Escape or
// defocused the window), the only Escape signal a browser hands us. Idempotent,
// since it fires every frame while the lock is gone; ignored while a death or
// victory screen owns the loop.
func (g *Game) Pause() {
	if g.dead || g.victory.Active() {
		return
	}
	g.pause.Show()
}

func (g *Game) Update() {
	g.ticks++

	// Death and victory overlays own the loop
	if g.dead {
		g.gameover.Update(tickSeconds)
		// A silhouette death swings the camera onto the figure as the overlay
		// fades in; a grab death already faced its foe during the catch.
		if g.deathBySilhouette {
			g.turnDeathGaze()
		}
		if input.JustPressed(input.Restart) {
			g.restart()
		}
		return
	}
	if g.victory.Active() {
		g.victory.Update(tickSeconds)
		if input.JustPressed(input.Confirm) {
			g.regenerate() // descend: clears the prompt and builds the next floor
		}
		return
	}

	// Pause menu: Space/Start toggles it; while open the whole world freezes. On
	// wasm the pause is also raised from main when Escape drops the pointer lock,
	// see Game.Pause.
	if input.JustPressed(input.Pause) {
		g.pause.Toggle()
	}
	if g.pause.Active() {
		if input.JustPressed(input.Restart) {
			g.restart()
		}
		return
	}

	g.updateDebug()
	g.disconnect.Update()
	g.info.Update(tickSeconds)

	// The white silhouette closes its run-wide gap while disconnected; when it
	// reaches the player the run ends.
	g.silhouette.Update(g.player.Pos, g.disconnect.Active())
	if g.silhouette.Dead() {
		g.dead = true
		g.deathBySilhouette = true
		g.gameover.Show(ui.DeathBySilhouette)
	}

	// Elevator level exit sequence
	if !g.exit.Active() {
		if e := g.entities.elevator; e.Active && e.Contains(g.player.Pos) {
			g.disconnect.ForceReconnect()
			g.exit.Start(e)
			g.pause.MarkEscape() // objective 2: reached the elevator and escaping
		}
	}
	if g.exit.Active() {
		g.exit.Update(g.player, g.camera)
		// Keep enemies moving/animating during the ride.
		g.entities.Update(g.entityContext())
		if g.exit.Done() {
			// Raise the victory prompt; the level change waits on the player's confirm.
			g.victory.Show(g.difficulty + 1)
		}
		g.updateCameraTransform()
		return
	}

	// An enemy catches the player to hit them
	if !g.catch.Active() && g.player.Invuln == 0 && !g.disconnect.Active() {
		if e := g.entities.CatchCandidate(g.player.Pos); e != nil {
			g.catch.Start(e)
		}
	}
	if g.catch.Active() {
		if g.catch.Update(g.player, g.camera) && g.player.HP <= 0 {
			g.dead = true
			g.gameover.Show(ui.DeathByDamage)
		}
		g.entities.Update(g.entityContext()) // keep the world (and the frozen grabber) ticking
		g.updateCameraTransform()
		return
	}
	if g.player.Invuln > 0 {
		g.player.Invuln--
	}

	// Camera look
	yaw, pitch := g.camera.YawPitch()
	yawoff, pitchoff := input.Look()
	yaw += yawoff
	pitch = max(min(pitch+pitchoff, math.Pi/2), -math.Pi/2)
	g.camera.SetYawPitch(yaw, pitch)

	// Movement input
	g.player.Running = input.Pressed(input.Run)
	g.player.Moving = input.Moving()
	g.player.Dark = g.disconnect.Active()
	g.player.Update()

	// Physics
	pos := input.ProcessMovement(
		g.player.Pos,
		g.camera.Direction(),
		g.camera.Right(),
		g.player.Momentum.Speed,
	)
	// Fold in any residual knockback from a hit, then let collision resolve it so
	// a shove into a wall stops naturally.
	pos = pos.Add(g.player.stepKnockback())
	// Switch level dimension
	if inpututil.IsKeyJustPressed(ebiten.KeyE) {
		//	g.levelIndex = (g.levelIndex + 1) % 2
	}

	// The player is pinned to the floor plane: no jumping, no vertical motion.
	pos[1] = 0.5 + PlayerHeight/2

	// Prevent going out of the map after a knockback
	start := g.player.Pos
	delta := pos.Sub(start)
	steps := 1
	if m := max(math.Abs(delta.X()), math.Abs(delta.Y()), math.Abs(delta.Z())); m > maxCollisionStep {
		steps = int(math.Ceil(m / maxCollisionStep))
	}
	inc := delta.Mul(1.0 / float64(steps))
	pos = start
	for range steps {
		pos = g.resolveWallCollision(pos.Add(inc))
	}

	// Hard safety net: clamp inside the level rectangle so the player can never end
	// up stranded off the map, whatever slips past the sweep above.
	lvl := g.levels[g.levelIndex]
	pos[0] = max(min(pos[0], float64(lvl.Width())-PlayerWidth/2), PlayerWidth/2)
	pos[2] = max(min(pos[2], float64(lvl.Depth())-PlayerWidth/2), PlayerWidth/2)
	g.player.Pos = pos

	g.entities.Update(g.entityContext())

	// Flash a banner the moment the button powers on the exit elevator.
	if on := g.entities.elevator.Active; on && !g.exitOn {
		g.info.Push("The exit elevator is online")
		g.pause.MarkButton() // objective 1: button found, elevator powered
	}
	g.exitOn = g.entities.elevator.Active

	g.updateCameraTransform()
}

// entityContext snapshots the per-tick world state the entity manager needs
// (player position, look direction, blackout, level for line-of-sight), built in
// one place so call sites just hand the manager a Context.
func (g *Game) entityContext() *entity.Context {
	return &entity.Context{
		PlayerPos:     g.player.Pos,
		ViewDir:       g.camera.Direction(),
		Disconnected:  g.disconnect.Active(),
		PlayerRunning: g.player.Running && g.player.Moving,
		World:         g.levels[g.levelIndex],
	}
}

// turnDeathGaze swings the frozen camera onto the white silhouette while the
// game-over overlay fades in, leveling the pitch so the death stare lands on the
// figure. Mirrors the catch sequence's forced look (CatchSequence.Update).
func (g *Game) turnDeathGaze() {
	sp := g.silhouette.Position()
	to := mgl64.Vec3{sp.X() - g.player.Pos.X(), 0, sp.Z() - g.player.Pos.Z()}
	yaw, pitch := g.camera.YawPitch()
	if to.Len() > 1e-6 {
		yaw = approachAngle(yaw, math.Atan2(to.X(), to.Z()), catchLookLerp)
	}
	pitch += (0 - pitch) * catchLookLerp
	g.camera.SetYawPitch(yaw, pitch)
	g.updateCameraTransform()
}

// updateCameraTransform syncs the camera to the player's FOV and bobbed render
// position. Used by both the normal loop and the exit sequence.
func (g *Game) updateCameraTransform() {
	g.camera.SetFov(g.player.Fov)
	renderPos := g.player.Pos.Add(mgl64.Vec3{0, g.player.Bob.Y, 0}).
		Add(g.camera.Right().Mul(g.player.Bob.X))
	g.camera.SetPosition(renderPos)
	g.camera.Update()

	// Sync the audio listener to the camera so positioned sounds pan/attenuate
	// relative to where the player stands and looks.
	xaudio.SetListener(
		[3]float64(g.camera.Position()),
		[3]float64(g.camera.Direction()),
		[3]float64(g.camera.Right()),
	)
}

func (g *Game) Ticks() int {
	return g.ticks
}
func (g *Game) Disconnect() *DisconnectState {
	return g.disconnect
}

func (g *Game) PlayerPosition() [3]float32 {
	p := g.player.Pos
	return [3]float32{float32(p.X()), float32(p.Y()), float32(p.Z())}
}

func (g *Game) Draw(screen *ebiten.Image) {
	pos := g.camera.Position()
	camFloor := floorVec3(pos)
	camFract := pos.Sub(camFloor)
	pvinv := g.camera.ProjectionMatrix().Mul4(g.camera.ViewMatrix()).Inv()

	vertices, indices := graphics.AppendRectVerticesIndices(nil, nil, 0, &graphics.RectOpts{
		SrcX:      0,
		SrcY:      0,
		DstWidth:  float32(screen.Bounds().Dx()),
		DstHeight: float32(screen.Bounds().Dy()),
		SrcWidth:  0,
		SrcHeight: 0,
		R:         1,
		G:         1,
		B:         1,
		A:         1,
	})
	lvl := g.levels[g.levelIndex]
	screen.DrawTrianglesShader(vertices, indices, assets.ShaderScene(), &ebiten.DrawTrianglesShaderOptions{
		Images: [4]*ebiten.Image{},
		Uniforms: map[string]any{
			"CamFloor":          camFloor[:],
			"CamFract":          camFract[:],
			"CameraPVMatrixInv": pvinv[:],
			"MapWidth":          lvl.Width(),
			"MapDepth":          lvl.Depth(),
			"Walls":             lvl.Data(),
			"Time":              float32(g.ticks) / logic.TPS,
			"Disconnect":        g.disconnect.T,
			"DisconnectDir":     g.disconnect.Dir,
		},
	})

	g.drawEntities(screen, pos, pvinv, camFloor, camFract, lvl)
	g.drawWhiteSilhouette(screen, pos)
	// g.drawMinimap(screen) // This is for debug
	g.drawUI(screen)
}

// drawUI lays the 2D overlay on top of the rendered frame: the HUD meters and
// info banner during play, replaced by the victory or game-over screen when one
// is raised.
func (g *Game) drawUI(screen *ebiten.Image) {
	if !g.victory.Active() && !g.gameover.Active() && !g.pause.Active() {
		g.hud.Draw(screen, float64(g.disconnect.Charge), g.silhouette.Budget(), g.player.HP, PlayerMaxHP)
		g.info.Draw(screen)
	}
	g.victory.Draw(screen)
	g.gameover.Draw(screen)
	g.pause.Draw(screen)
}

func (g *Game) drawWhiteSilhouette(screen *ebiten.Image, camPos mgl64.Vec3) {
	// Hold off until the blackout is well underway
	reveal := smoothstep(0.55, 0.95, g.disconnect.T)
	// On a silhouette death the blackout is frozen mid-fade; force full reveal so
	// the death gaze lands on a clearly visible figure.
	if g.deathBySilhouette {
		reveal = 1
	}
	if reveal <= 0 {
		return
	}
	pv := g.camera.ProjectionMatrix().Mul4(g.camera.ViewMatrix())
	right := g.camera.Right()
	sw, sh := screen.Bounds().Dx(), screen.Bounds().Dy()

	vx, ix := g.silhouette.AppendGeometry(nil, nil, camPos, right, &pv, sw, sh)
	if len(vx) == 0 {
		return
	}
	// AppendGeometry leaves vertex colour to us: paint every vertex white at the
	// reveal alpha so it blends up out of the blackout.
	for i := range vx {
		vx[i].ColorR, vx[i].ColorG, vx[i].ColorB, vx[i].ColorA = 1, 1, 1, reveal
	}
	screen.DrawTriangles(vx, ix, graphics.BrushImage, &ebiten.DrawTrianglesOptions{AntiAlias: true})
}

// smoothstep is the standard Hermite ramp from 0 at e0 to 1 at e1.
func smoothstep(e0, e1, x float32) float32 {
	t := (x - e0) / (e1 - e0)
	t = max(0, min(1, t))
	return t * t * (3 - 2*t)
}

// drawEntities renders all silhouettes/structures after the voxel scene
func (g *Game) drawEntities(screen *ebiten.Image, camPos mgl64.Vec3, pvinv mgl64.Mat4, camFloor, camFract mgl64.Vec3, lvl *level.Level) {
	if g.disconnect.T >= 1 {
		return
	}
	pv := g.camera.ProjectionMatrix().Mul4(g.camera.ViewMatrix())
	right := g.camera.Right()
	sw, sh := screen.Bounds().Dx(), screen.Bounds().Dy()

	vx, ix := g.entities.AppendGeometry(nil, nil, camPos, right, &pv, sw, sh)
	if len(vx) == 0 {
		return
	}
	screen.DrawTrianglesShader(vx, ix, assets.ShaderEntity(), &ebiten.DrawTrianglesShaderOptions{
		Images: [4]*ebiten.Image{},
		Uniforms: map[string]any{
			"CamFloor":          camFloor[:],
			"CamFract":          camFract[:],
			"CameraPVMatrixInv": pvinv[:],
			"MapWidth":          lvl.Width(),
			"MapDepth":          lvl.Depth(),
			"Walls":             lvl.Data(),
			"Time":              float32(g.ticks) / logic.TPS,
			"Disconnect":        g.disconnect.T,
			"DisconnectDir":     g.disconnect.Dir,
		},
	})
}
