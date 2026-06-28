package audio

import (
	"encoding/binary"
	"io"
	"math"
	"math/rand"
	"time"
)

// Intervals derived from core.bobWalkFreq/bobRunFreq at TPS=60 so footfalls
// stay in sync with the head bob: 2π / (freq * TPS).
const (
	walkStepInterval = 0.5512 // seconds between footfalls at walk speed
	runStepInterval  = 0.3272 // seconds between footfalls at run speed
	footstepTail     = 1.5    // seconds a footstep keeps ringing after its onset
)

// Footstep generates walk/run footstep sounds.
// Set Active when the player is moving, Blend 0=walk 1=run.
type Footstep struct {
	t          float64
	stepT      float64
	foot       int // alternates 0/1 for L/R foot
	rng        *rand.Rand
	started    bool    // a footstep has been triggered at least once
	clickLP    float64 // one-pole low-pass state for click noise
	thumpPhase float64 // randomized each step, kept in [π/4, 3π/4] to avoid zero-crossings
	stepAmp    float64 // per-step amplitude scaler
	stepFreq   float64 // per-step frequency scaler

	Active        bool
	Blend         float64 // 0=walk, 1=run
	IntervalScale float64 // stretches step spacing (dark movement = slower); 1 = normal
}

func NewFootstep() *Footstep {
	// Start "armed" so the first move steps immediately rather than after a full interval.
	return &Footstep{
		rng:           rand.New(rand.NewSource(time.Now().Unix())),
		stepAmp:       1,
		stepFreq:      1,
		stepT:         footstepTail,
		IntervalScale: 1,
	}
}

// NewFootstepWithReverb returns the Footstep controller and a reverb-wrapped reader ready for NewPlayer.
func NewFootstepWithReverb(wet float64) (*Footstep, io.Reader) {
	f := NewFootstep()
	return f, NewReverb(f, wet)
}

// stepInterval is the seconds between footfalls: the walk..run cadence by Blend,
// stretched by IntervalScale so dark (slower) movement spaces steps further apart.
func (f *Footstep) stepInterval() float64 {
	scale := f.IntervalScale
	if scale <= 0 {
		scale = 1
	}
	return (walkStepInterval + (runStepInterval-walkStepInterval)*f.Blend) * scale
}

func (f *Footstep) Read(buf []byte) (int, error) {
	const dt = 1.0 / sampleRate
	n := len(buf) / 4

	for i := 0; i < n; i++ {
		f.t += dt
		f.stepT += dt

		// stepT counts continuously, even while idle, so spamming movement keys
		// can't machine-gun footsteps: a fresh step fires only once a full cadence
		// interval of real time has elapsed since the last onset.
		if f.Active {
			interval := f.stepInterval()
			if f.stepT >= interval {
				f.stepT = 0
				f.started = true
				f.foot ^= 1
				// Pick a random phase in [π/4, 3π/4]: sin is always ≥ 0.707 at onset.
				f.thumpPhase = math.Pi/4 + f.rng.Float64()*math.Pi/2
				f.stepAmp = 0.85 + f.rng.Float64()*0.30  // ±15% volume
				f.stepFreq = 0.88 + f.rng.Float64()*0.24 // ±12% pitch
			}
		} else if f.stepT > footstepTail {
			// Idle: stop counting once the step has fully rung out, but keep stepT
			// past every interval so the next move steps immediately.
			f.stepT = footstepTail
		}

		// Render the current step's envelope, letting it ring out naturally even
		// after the key is released so footsteps are never cut off mid-thump.
		var sample float64
		if f.started && f.stepT < footstepTail {
			tss := f.stepT // time since last step onset

			// Slight pitch alternation between feet for naturalness
			thumpFreq := 10 + float64(f.foot)*10

			thumpAmp := 0.8 - 0.02*f.Blend
			thumpDecay := 5 + 10.0*f.Blend
			clickAmp := 0.1 + 0.05*f.Blend
			clickDecay := 60.0 + 20.0*f.Blend

			thump := math.Sin(2*math.Pi*thumpFreq*f.stepFreq*tss+f.thumpPhase) * thumpAmp * f.stepAmp * math.Exp(-thumpDecay*tss)
			// Low-pass the noise at ~2kHz to remove hi-hat quality
			rawClick := (f.rng.Float64()*2 - 1) * clickAmp * math.Exp(-clickDecay*tss)
			f.clickLP = 0.05*f.clickLP + 0.4*rawClick
			click := f.clickLP

			sample = thump + click
		}

		if sample > 1 {
			sample = 1
		}
		if sample < -1 {
			sample = -1
		}

		v := int16(sample * math.MaxInt16)
		off := i * 4
		binary.LittleEndian.PutUint16(buf[off:], uint16(v))
		binary.LittleEndian.PutUint16(buf[off+2:], uint16(v))
	}

	return n * 4, nil
}
