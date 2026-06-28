package audio

import (
	"encoding/binary"
	"io"
	"math"
	"sync/atomic"

	"github.com/hajimehoshi/ebiten/v2/audio"
)

// Spatialization tunables. Distances are in world units (same space as the
// player/entity positions).
const (
	refDistance = 4.0  // within this radius the sound plays at full volume
	maxDistance = 64.0 // attenuation/muffle are clamped at this radius
	rolloff     = 1.0  // higher = volume drops off faster with distance

	muffleFloor = 0.18 // lowest low-pass coefficient (most muffled, far away)
	backMuffle  = 0.25 // extra muffle for sounds directly behind the listener
)

// The listener (player ears) in world space. Written once per frame from the
// game goroutine via SetListener; read by recompute on the same goroutine, so
// no synchronisation is needed here.
var (
	listenerPos   [3]float64
	listenerFwd   = [3]float64{0, 0, -1}
	listenerRight = [3]float64{1, 0, 0}

	spatialPlayers []*SpatialPlayer
)

// SetListener updates the listener pose and refreshes every live spatial player
// so static sounds re-pan as the player turns or moves. forward and right are
// the listener's orientation axes (e.g. camera Direction and Right).
func SetListener(pos, forward, right [3]float64) {
	listenerPos, listenerFwd, listenerRight = pos, forward, right
	for _, sp := range spatialPlayers {
		sp.recompute()
	}
}

// SpatialPlayer wraps an *audio.Player and positions its sound in the world.
// Distance attenuation drives the wrapped player's volume; left/right balance
// and distance muffle are applied by the underlying panStream.
type SpatialPlayer struct {
	player  *audio.Player
	stream  *panStream
	pos     [3]float64
	baseVol float64
}

// NewSpatialPlayer turns any seekable stereo-PCM source into a positioned
// player. src must decode to 16-bit little-endian stereo (wav.Decode* and the
// synth helpers already do); Seek enables Rewind for one-shot replays.
func NewSpatialPlayer(src io.ReadSeeker) (*SpatialPlayer, error) {
	ps := &panStream{src: src}
	p, err := ctx.NewPlayer(ps)
	if err != nil {
		return nil, err
	}
	sp := &SpatialPlayer{player: p, stream: ps, baseVol: 1}
	spatialPlayers = append(spatialPlayers, sp)
	sp.recompute()
	return sp, nil
}

// SetBaseVolume scales the sound before distance attenuation (0..1).
func (sp *SpatialPlayer) SetBaseVolume(v float64) {
	sp.baseVol = v
	sp.recompute()
}

// SetPosition moves the sound's world position and refreshes its spatial mix.
// Call each frame for moving emitters, or once for a fixed one.
func (sp *SpatialPlayer) SetPosition(pos [3]float64) {
	sp.pos = pos
	sp.recompute()
}

// PlayAt positions the sound and (re)starts it from the beginning. Mirrors the
// Rewind+Play pattern used by the non-spatial sfx.
func (sp *SpatialPlayer) PlayAt(pos [3]float64) {
	sp.SetPosition(pos)
	sp.player.Rewind()
	sp.player.Play()
}

func (sp *SpatialPlayer) Play()             { sp.player.Play() }
func (sp *SpatialPlayer) Pause()            { sp.player.Pause() }
func (sp *SpatialPlayer) Rewind()           { sp.player.Rewind() }
func (sp *SpatialPlayer) IsPlaying() bool   { return sp.player.IsPlaying() }
func (sp *SpatialPlayer) Player() *audio.Player { return sp.player }

// recompute derives volume (attenuation), L/R pan gains and the low-pass
// coefficient from the source position relative to the current listener.
func (sp *SpatialPlayer) recompute() {
	dx := sp.pos[0] - listenerPos[0]
	dy := sp.pos[1] - listenerPos[1]
	dz := sp.pos[2] - listenerPos[2]
	dist := math.Sqrt(dx*dx + dy*dy + dz*dz)

	// On top of the listener: dead-centre, full volume, no muffle.
	if dist < 1e-4 {
		sp.player.SetVolume(sp.baseVol)
		sp.stream.set(math.Sqrt2/2, math.Sqrt2/2, 1)
		return
	}

	// Distance attenuation (inverse-distance, clamped) -> player volume.
	d := math.Min(math.Max(dist, refDistance), maxDistance)
	att := refDistance / (refDistance + rolloff*(d-refDistance))
	sp.player.SetVolume(sp.baseVol * att)

	inv := 1 / dist
	nx, ny, nz := dx*inv, dy*inv, dz*inv

	// Constant-power stereo pan from the right-axis component (-1 left..1 right).
	pan := math.Min(math.Max(nx*listenerRight[0]+ny*listenerRight[1]+nz*listenerRight[2], -1), 1)
	a := (pan + 1) * math.Pi / 4
	gainL, gainR := math.Cos(a), math.Sin(a)

	// Distance muffle, plus extra damping for sounds behind the listener.
	norm := (d - refDistance) / (maxDistance - refDistance)
	k := 1 - norm*(1-muffleFloor)
	if fwd := nx*listenerFwd[0] + ny*listenerFwd[1] + nz*listenerFwd[2]; fwd < 0 {
		k *= 1 + backMuffle*fwd // fwd in [-1,0): mild low-pass for the back hemisphere
	}
	sp.stream.set(gainL, gainR, math.Min(math.Max(k, muffleFloor), 1))
}

// panStream applies per-channel gain and a one-pole low-pass to a stereo
// 16-bit PCM source. Read runs on the audio goroutine while the game goroutine
// updates the parameters, so gain/lowpass are passed through atomics; the
// filter state (yl/yr) is touched only by Read.
type panStream struct {
	src     io.ReadSeeker
	gainL   atomic.Uint64 // float64 bits
	gainR   atomic.Uint64 // float64 bits
	lowpass atomic.Uint64 // float64 bits, 1 = no filtering
	yl, yr  float64
}

func (s *panStream) set(gainL, gainR, lowpass float64) {
	s.gainL.Store(math.Float64bits(gainL))
	s.gainR.Store(math.Float64bits(gainR))
	s.lowpass.Store(math.Float64bits(lowpass))
}

func (s *panStream) Read(b []byte) (int, error) {
	n, err := s.src.Read(b)
	gl := math.Float64frombits(s.gainL.Load())
	gr := math.Float64frombits(s.gainR.Load())
	k := math.Float64frombits(s.lowpass.Load())
	for i := 0; i+4 <= n; i += 4 {
		xl := float64(int16(binary.LittleEndian.Uint16(b[i:])))
		xr := float64(int16(binary.LittleEndian.Uint16(b[i+2:])))
		s.yl += k * (xl - s.yl)
		s.yr += k * (xr - s.yr)
		binary.LittleEndian.PutUint16(b[i:], uint16(clampI16(s.yl*gl)))
		binary.LittleEndian.PutUint16(b[i+2:], uint16(clampI16(s.yr*gr)))
	}
	return n, err
}

func (s *panStream) Seek(offset int64, whence int) (int64, error) {
	s.yl, s.yr = 0, 0 // drop filter state so a rewound replay starts clean
	return s.src.Seek(offset, whence)
}

func clampI16(v float64) int16 {
	if v > math.MaxInt16 {
		return math.MaxInt16
	}
	if v < math.MinInt16 {
		return math.MinInt16
	}
	return int16(v)
}
