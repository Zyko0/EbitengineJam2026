package audio

import (
	"encoding/binary"
	"math"
	"math/rand"
)

// Skitter synthesises the dry, sibilant scuttle of the spider's many legs
type Skitter struct {
	rng *rand.Rand
	bp  resonator // bandpass colouring the noise; re-tuned per tap

	gap   int     // samples until the next leg-tap onset
	env   float64 // current tap amplitude envelope
	decay float64 // per-sample envelope multiplier for the active tap
	burst int     // taps left in the current rapid cluster

	hpX, hpY float64 // one-pole high-pass (DC-blocker form) state
}

// NewSkitter seeds an independent scuttle. There is only one spider, but seeding
// keeps the pattern varied run to run.
func NewSkitter(seed int64) *Skitter {
	return &Skitter{rng: rand.New(rand.NewSource(seed))}
}

func (s *Skitter) Read(buf []byte) (int, error) {
	n := len(buf) / 4
	for i := 0; i < n; i++ {
		if s.gap <= 0 {
			s.trigger()
		}
		s.gap--

		white := s.rng.Float64()*2 - 1
		body := s.bp.process(white)
		// One-pole high-pass: y = x - x_prev + R*y_prev (R<1). Strips rumble and
		// leaves the airy hiss that sells the "sss".
		s.hpY = white - s.hpX + 0.96*s.hpY
		s.hpX = white

		v := (body*0.7 + s.hpY*0.35) * s.env
		s.env *= s.decay

		if v > 1 {
			v = 1
		} else if v < -1 {
			v = -1
		}
		val := int16(v * math.MaxInt16)
		off := i * 4
		binary.LittleEndian.PutUint16(buf[off:], uint16(val))
		binary.LittleEndian.PutUint16(buf[off+2:], uint16(val))
	}

	return n * 4, nil
}

// trigger advances the sequencer: between clusters it just waits a beat, then
// fires the taps of a cluster one by one, each a fresh band-passed noise burst.
func (s *Skitter) trigger() {
	if s.burst <= 0 {
		// Pause between clusters, then arm the next run of rapid taps.
		s.burst = 2 + s.rng.Intn(5)                             // 2..6 taps per run
		s.gap = int((0.06 + s.rng.Float64()*0.12) * sampleRate) // 60..180ms gap
		return
	}
	s.burst--

	// Half the taps are sharp bright "tch" ticks, the rest softer sibilant "ss".
	var freq, dec, amp, hold float64
	if s.rng.Float64() < 0.5 {
		freq = 3500 + s.rng.Float64()*3000 // bright
		dec = 90 + s.rng.Float64()*60      // very fast -> "tch"
		amp = 0.6 + s.rng.Float64()*0.4
		hold = 0.012 + s.rng.Float64()*0.02 // tight spacing within the cluster
	} else {
		freq = 5000 + s.rng.Float64()*2000
		dec = 35 + s.rng.Float64()*25 // softer hiss -> "sss"
		amp = 0.4 + s.rng.Float64()*0.3
		hold = 0.02 + s.rng.Float64()*0.04
	}
	s.bp.setFreq(freq, freq*0.6) // wide bandwidth keeps it noisy, not tonal
	s.env = amp
	s.decay = math.Exp(-dec / sampleRate)
	s.gap = int(hold * sampleRate)
}

// Seek restarts the pattern; the stream is endless so it never reports EOF.
func (s *Skitter) Seek(offset int64, whence int) (int64, error) {
	if offset == 0 {
		s.gap, s.burst = 0, 0
		s.env, s.decay = 0, 0
		s.hpX, s.hpY = 0, 0
	}
	return 0, nil
}
