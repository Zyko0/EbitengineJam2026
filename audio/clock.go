package audio

import (
	"encoding/binary"
	"math"
	"math/rand"
)

// Procedural clock tic/tac: a sharp resonant click fed through a small
// Schroeder reverb so it reads as a clock ticking in a room. Tic and tac differ
// only in pitch, mirroring the high/low of a mechanical escapement.
const (
	clickDur   = 0.06 // seconds of dry transient
	reverbTail = 0.20 // seconds of reverb decay appended after the click

	ticFreq = 2200.0 // bright "tic"
	tacFreq = 1500.0 // duller "tac"
)

// synthClock renders one click at freq into stereo 16-bit PCM bytes.
func synthClock(freq float64) []byte {
	dry := synthClick(freq)
	wet := reverb(dry)
	return toPCM16(wet)
}

// synthClick builds the dry transient: a noise burst gives the mechanical snap,
// a damped resonant sine gives the tonal body, both under a fast decay.
func synthClick(freq float64) []float64 {
	n := int(clickDur * sampleRate)
	out := make([]float64, n)
	rng := rand.New(rand.NewSource(int64(freq)))
	for i := range out {
		t := float64(i) / sampleRate
		body := math.Sin(2*math.Pi*freq*t) * math.Exp(-t*200)
		snap := (rng.Float64()*2 - 1) * math.Exp(-t*350)
		out[i] = 0.35*body + 0.65*snap
	}
	return out
}

// reverb applies a simple Schroeder reverb (parallel comb filters into a series
// allpass) and appends a decaying tail to the buffer.
func reverb(dry []float64) []float64 {
	n := len(dry) + int(reverbTail*sampleRate)
	x := make([]float64, n)
	copy(x, dry)

	// Comb delays (~25-37ms) chosen mutually prime to avoid metallic ringing.
	combDelay := []int{1116, 1188, 1277, 1356}
	const combFB = 0.6
	out := make([]float64, n)
	for _, d := range combDelay {
		buf := make([]float64, n)
		for i := range x {
			v := x[i]
			if i >= d {
				v += combFB * buf[i-d]
			}
			buf[i] = v
			out[i] += v
		}
	}
	for i := range out {
		out[i] *= 0.25
	}

	// Allpass diffusion to smear the comb output.
	const apDelay = 556
	const apFB = 0.7
	ap := make([]float64, n)
	for i := range out {
		v := out[i]
		var delayed float64
		if i >= apDelay {
			delayed = ap[i-apDelay]
			v += apFB * delayed
		}
		ap[i] = v
		out[i] = -apFB*v + delayed
	}

	// Mix dry click back in so the attack stays crisp, then normalise.
	for i := range dry {
		out[i] += dry[i]
	}
	peak := 0.0
	for _, v := range out {
		if a := math.Abs(v); a > peak {
			peak = a
		}
	}
	if peak > 0 {
		g := 0.9 / peak
		for i := range out {
			out[i] *= g
		}
	}
	return out
}

// toPCM16 encodes mono float samples as little-endian stereo 16-bit PCM.
func toPCM16(samples []float64) []byte {
	buf := make([]byte, len(samples)*4)
	for i, v := range samples {
		s := int16(math.Round(math.Max(-1, math.Min(1, v)) * 32767))
		binary.LittleEndian.PutUint16(buf[i*4:], uint16(s))
		binary.LittleEndian.PutUint16(buf[i*4+2:], uint16(s))
	}
	return buf
}
