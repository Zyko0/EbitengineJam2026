package audio

import (
	"bytes"
	"io"
	"log"
	"time"

	"github.com/Zyko0/EbitengineJam2026/assets"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/wav"
)

const sampleRate = 44100

var (
	ctx *audio.Context

	pushButtonPlayer    *SpatialPlayer
	elevatorLeavePlayer *SpatialPlayer
	tallguyLaughPlayer  *SpatialPlayer
	ticPlayer           *SpatialPlayer
	tacPlayer           *SpatialPlayer
	hitPlayer           *SpatialPlayer

	// Looping ambience players. One instance each, reused across levels and game
	// states so re-entering a floor (restart, next floor) never stacks a second
	// copy of the loop. A reset pauses them all (StopLoops); the new level's
	// entities resume only the ones they own, leaving any unused loop silent.
	spiderSkitterPlayer *SpatialPlayer
	voicePlayers        = map[Voice]*SpatialPlayer{}
	loopPlayers         []*SpatialPlayer // every looping player, for StopLoops

	clockTac bool // alternates tic/tac per blink event
)

// newSpatialWav decodes a wav asset and wraps it in a positioned player.
func newSpatialWav(b []byte) *SpatialPlayer {
	wavReader, err := wav.DecodeWithSampleRate(sampleRate, bytes.NewReader(b))
	if err != nil {
		log.Fatal(err)
	}
	sp, err := NewSpatialPlayer(wavReader)
	if err != nil {
		log.Fatal(err)
	}
	return sp
}

func init() {
	ctx = audio.NewContext(sampleRate)

	pushButtonPlayer = newSpatialWav(assets.ButtonPushAudioBytes)
	pushButtonPlayer.SetBaseVolume(0.75)

	elevatorLeavePlayer = newSpatialWav(assets.ElevatorLeaveAudioBytes)
	tallguyLaughPlayer = newSpatialWav(assets.TallguyLaughAudioBytes)
	hitPlayer = newSpatialWav(assets.HitAudioBytes)

	// Clock tic/tac sfx (procedurally synthesised, reverb baked in).
	var err error
	ticPlayer, err = NewSpatialPlayer(bytes.NewReader(synthClock(ticFreq)))
	if err != nil {
		log.Fatal(err)
	}
	tacPlayer, err = NewSpatialPlayer(bytes.NewReader(synthClock(tacFreq)))
	if err != nil {
		log.Fatal(err)
	}

	// The lone ceiling spider's scuttle loop.
	spiderSkitterPlayer, err = NewSpatialPlayer(NewSkitter(1))
	if err != nil {
		log.Fatal(err)
	}
	spiderSkitterPlayer.SetBaseVolume(0.6)
	registerLoop(spiderSkitterPlayer)
}

// registerLoop tracks a looping player so StopLoops can silence it on a reset.
func registerLoop(sp *SpatialPlayer) {
	loopPlayers = append(loopPlayers, sp)
}

// StopLoops pauses every looping ambience player. Call it on a game state reset
// (restart, next floor) before the new level's entities claim the loops they
// need, so loops never stack and a loop with no owner on the new floor stays
// silent.
func StopLoops() {
	for _, sp := range loopPlayers {
		sp.Pause()
	}
}

// Patroller voice presets. Pitch sets the glottal fundamental and FormantScale
// the vocal-tract size; Rate sets the speech tempo. Use these to tell the two
// patroller types apart.
var (
	VoiceDeep = Voice{
		FormantScale: 0.85,
		Rate:         0.9,
		Pitch:        95,
	} // larger, slower, low murmur
	VoiceLight = Voice{
		FormantScale: 1.18,
		Rate:         1.1,
		Pitch:        150,
	} // smaller, quicker, higher voice
)

// PatrollerVoice returns the shared looping whispered-gibberish player for the
// given timbre preset. One instance per preset is created on first use and
// reused across levels, so re-entering a floor never stacks the voice. Call Play
// once, then SetPosition each frame with the patroller's world position so the
// spatial pan/attenuation cue where it is and whether it's approaching. There is
// only ever one patroller per preset alive at a time.
func PatrollerVoice(voice Voice) *SpatialPlayer {
	if sp := voicePlayers[voice]; sp != nil {
		return sp
	}
	// Seed from the timbre so the two preset voices generate distinct gibberish;
	// the preset's pitch/formant/rate do the rest of the differentiating.
	sp, err := NewSpatialPlayer(NewGibberish(int64(voice.Pitch*100), voice))
	if err != nil {
		log.Fatal(err)
	}
	sp.SetBaseVolume(1.25)
	voicePlayers[voice] = sp
	registerLoop(sp)

	return sp
}

// SpiderSkitter returns the shared looping scuttle player for the lone ceiling
// spider. Reused across levels. Call Play once and SetPosition each frame with
// the spider's world position so the spatial pan/attenuation cue where it
// crawls; Pause it while the spider holds still so the hiss only sounds when it
// moves.
func SpiderSkitter() *SpatialPlayer {
	return spiderSkitterPlayer
}

func NewPlayer(src io.Reader) (*audio.Player, error) {
	p, err := ctx.NewPlayer(src)
	if err != nil {
		return nil, err
	}
	p.SetBufferSize(50 * time.Millisecond)

	return p, nil
}

// Audio assets. Each takes the sound's world position so it is mixed relative
// to the listener set via SetListener.

func PlayButtonPush(pos [3]float64) {
	pushButtonPlayer.PlayAt(pos)
}

func PlayElevatorLeave(pos [3]float64) {
	elevatorLeavePlayer.PlayAt(pos)
}

func PlayTallguyLaugh(pos [3]float64) {
	tallguyLaughPlayer.PlayAt(pos)
}

func PlayTic(pos [3]float64) {
	ticPlayer.PlayAt(pos)
}

func PlayTac(pos [3]float64) {
	tacPlayer.PlayAt(pos)
}

func PlayHit(pos [3]float64) {
	hitPlayer.PlayAt(pos)
}

// PlayClockTick alternates tic/tac so successive blink events read as a ticking
// clock. Call it once per tall guy blink toggle, with the clock's position.
func PlayClockTick(pos [3]float64) {
	if clockTac {
		PlayTac(pos)
	} else {
		PlayTic(pos)
	}
	clockTac = !clockTac
}
