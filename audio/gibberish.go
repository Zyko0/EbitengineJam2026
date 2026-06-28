package audio

import (
	"encoding/binary"
	"math"
	"math/rand"
)

// Gibberish synthesises endless nonsense speech. A glottal buzz (a sawtooth at
// a fundamental) is shaped by a bank of formant resonators into pitched vowels;
// a small state machine sequences vowels, brief silent stops, words and pauses
// so it reads as someone muttering just out of sight. Wrap it in a SpatialPlayer
// so distance/pan cue where the patroller is.
//
// It implements io.ReadSeeker (Seek resets the utterance) and emits 16-bit
// little-endian stereo PCM, like the other synth streams.

// Vowel formant triplets (F1,F2,F3 in Hz). Each gives a different vowel colour.
var vowelFormants = [][3]float64{
	{730, 1090, 2440}, // "ah"
	{270, 2290, 3010}, // "ee"
	{300, 870, 2240},  // "oo"
	{530, 1840, 2480}, // "eh"
	{640, 1190, 2390}, // "uh"
	{390, 1990, 2550}, // "ih"
}

// controlPeriod is how often (in samples) formant glides and filter
// coefficients are recomputed. ~1.5ms is smooth enough and keeps the
// transcendentals off the per-sample path.
const controlPeriod = 64

// segment kinds the sequencer steps through.
type segment int

const (
	segVowel segment = iota
	segConsonant
	segPause
)

// Voice distinguishes one speaker from another. Pitch is the glottal
// fundamental in Hz (lower is deeper). FormantScale scales the formant
// frequencies (vocal-tract length): <1 is larger/deeper, >1 is smaller/higher.
// Rate multiplies the speech tempo. FormantScale and Rate default to 1, Pitch
// to 120Hz.
type Voice struct {
	FormantScale float64
	Rate         float64
	Pitch        float64
}

type Gibberish struct {
	rng   *rand.Rand
	voice Voice
	res   [3]resonator // parallel formant bank

	seg    segment
	segLen int // total samples in the current segment
	segPos int // samples elapsed in the current segment
	ctrl   int // samples until the next control-rate update

	curF, tgtF [3]float64 // current (glided) and target formant freqs
	phase      float64    // glottal oscillator phase [0,1)
	f0         float64    // current fundamental (Hz), incl. vibrato/contour
	pitchScale float64    // per-word pitch contour (rises on questions)
	ctrlT      float64    // control-rate clock, drives the vibrato

	wordSyl  int  // vowels in the current word
	sylLeft  int  // vowels remaining in the current word
	question bool // current phrase ends on a rising (inquisitive) contour
}

// NewGibberish seeds an independent voice; pass e.g. a patroller index so
// concurrent patrollers don't mutter in lockstep.
func NewGibberish(seed int64, voice Voice) *Gibberish {
	if voice.FormantScale == 0 {
		voice.FormantScale = 1
	}
	if voice.Rate == 0 {
		voice.Rate = 1
	}
	if voice.Pitch == 0 {
		voice.Pitch = 120
	}
	g := &Gibberish{rng: rand.New(rand.NewSource(seed)), voice: voice}
	g.startWord()
	g.curF = g.tgtF // start settled so the first vowel doesn't sweep up from 0
	for j := range g.res {
		g.res[j].setFreq(g.curF[j], formantBW(g.curF[j]))
	}

	return g
}

func (g *Gibberish) Read(buf []byte) (int, error) {
	n := len(buf) / 4
	for i := 0; i < n; i++ {
		if g.segPos >= g.segLen {
			g.nextSegment()
		}
		if g.ctrl <= 0 {
			g.updateControl()
			g.ctrl = controlPeriod
		}
		g.ctrl--

		s := g.synth()
		g.segPos++

		if s > 1 {
			s = 1
		} else if s < -1 {
			s = -1
		}
		v := int16(s * math.MaxInt16)
		off := i * 4
		binary.LittleEndian.PutUint16(buf[off:], uint16(v))
		binary.LittleEndian.PutUint16(buf[off+2:], uint16(v))
	}

	return n * 4, nil
}

// Seek restarts the utterance; the stream is endless so it never reports EOF.
func (g *Gibberish) Seek(offset int64, whence int) (int64, error) {
	if offset == 0 {
		g.startWord()
		g.curF = g.tgtF
		g.segPos, g.ctrl = 0, 0
	}
	return 0, nil
}

// synth renders one sample for the active segment.
func (g *Gibberish) synth() float64 {
	frac := float64(g.segPos) / float64(g.segLen)
	switch g.seg {
	case segVowel:
		// Glottal buzz: a sawtooth at f0 is harmonically rich, so the formants
		// carve it into a pitched vowel.
		if g.phase += g.f0 / sampleRate; g.phase >= 1 {
			g.phase--
		}
		saw := 2*g.phase - 1
		v := g.res[0].process(saw) + 0.7*g.res[1].process(saw) + 0.4*g.res[2].process(saw)
		return v * math.Sin(math.Pi*frac) * 0.4 // per-syllable amplitude hump
	default: // segConsonant / segPause: brief silences between syllables / words
		return 0
	}
}

// updateControl glides the formants toward their target and refreshes the
// resonator coefficients at the control rate.
func (g *Gibberish) updateControl() {
	const glide = 0.25
	for j := range g.curF {
		g.curF[j] += (g.tgtF[j] - g.curF[j]) * glide
		g.res[j].setFreq(g.curF[j], formantBW(g.curF[j]))
	}
	// Fundamental with a gentle 5Hz vibrato so the buzz isn't robotic.
	g.ctrlT += float64(controlPeriod) / sampleRate
	g.f0 = g.voice.Pitch * g.pitchScale * (1 + 0.03*math.Sin(2*math.Pi*5*g.ctrlT))
}

// nextSegment advances the word/phrase state machine.
func (g *Gibberish) nextSegment() {
	switch g.seg {
	case segVowel:
		if g.sylLeft--; g.sylLeft > 0 {
			g.startConsonant()
		} else {
			g.startPause()
		}
	case segConsonant:
		g.startVowel()
	case segPause:
		g.startWord()
	}
	g.segPos = 0
}

func (g *Gibberish) startWord() {
	g.wordSyl = 2 + g.rng.Intn(4) // 2..5 syllables
	g.sylLeft = g.wordSyl
	g.question = g.rng.Float64() < 0.25
	g.startVowel()
}

func (g *Gibberish) startVowel() {
	g.seg = segVowel
	g.segLen = g.dur(0.06 + g.rng.Float64()*0.08) // 60..140ms
	v := vowelFormants[g.rng.Intn(len(vowelFormants))]
	prog := 1 - float64(g.sylLeft)/float64(g.wordSyl) // 0 early .. ~1 late in the word
	// Pitch contour: questions rise toward the end, statements drift down.
	if g.question {
		g.pitchScale = 1 + 0.18*prog
	} else {
		g.pitchScale = 1 - 0.05*prog
	}
	// Speaker timbre, plus a touch of formant brightening on questions.
	scale := g.voice.FormantScale
	if g.question {
		scale *= 1 + 0.10*prog
	}
	g.tgtF = [3]float64{v[0] * scale, v[1] * scale, v[2] * scale}
}

func (g *Gibberish) startConsonant() {
	g.seg = segConsonant
	g.segLen = g.dur(0.03 + g.rng.Float64()*0.03) // 30..60ms silent stop between syllables
}

func (g *Gibberish) startPause() {
	g.seg = segPause
	sec := 0.10 + g.rng.Float64()*0.08 // between words
	if g.rng.Float64() < 0.25 {
		sec = 0.40 + g.rng.Float64()*0.30 // longer "thinking" pause
	}
	g.segLen = g.dur(sec)
}

// dur converts a length in seconds to samples, scaled by the voice's speech rate.
func (g *Gibberish) dur(sec float64) int {
	return int(sec / g.voice.Rate * sampleRate)
}

// formantBW widens the resonator bandwidth with centre frequency, as in real
// vocal tracts; keeps high formants from ringing too purely.
func formantBW(f float64) float64 {
	return 60 + 0.1*f
}

// resonator is a two-pole bandpass: y[n] = a0*x[n] - a1*y[n-1] - a2*y[n-2],
// tuned to a centre frequency and bandwidth. a0 normalises the peak to ~unity.
type resonator struct {
	a0, a1, a2 float64
	y1, y2     float64
}

func (r *resonator) setFreq(f, bw float64) {
	rr := math.Exp(-math.Pi * bw / sampleRate)
	theta := 2 * math.Pi * f / sampleRate
	r.a1 = -2 * rr * math.Cos(theta)
	r.a2 = rr * rr
	r.a0 = (1 - rr*rr) * math.Sin(theta)
}

func (r *resonator) process(x float64) float64 {
	y := r.a0*x - r.a1*r.y1 - r.a2*r.y2
	r.y2 = r.y1
	r.y1 = y

	return y
}
