package fir

import (
	"math"
)

func MakeHighPass(gain, sampleRate, cutFrequency, transitionWidth float64, winType WindowType) []float32 {
	// var nTaps = computeNTaps(sampleRate, transitionWidth)
	nTaps := computeNTaps(sampleRate, transitionWidth, winType, 0.0)
	// log.Println("Taps: ", nTaps)
	var taps = make([]float32, nTaps)
	var w = HammingWindow(nTaps)

	var M = (nTaps - 1) / 2
	var fwT0 = 2 * math.Pi * cutFrequency / sampleRate

	for i := -M; i <= M; i++ {
		if i == 0 {
			taps[i+M] = float32((1 - fwT0/math.Pi) * float64(w[i+M]))
		} else {
			taps[i+M] = float32(-math.Sin(float64(i)*fwT0) / (float64(i) * math.Pi) * float64(w[i+M]))
		}
	}

	var fmax = float64(taps[0+M])
	for i := 1; i <= M; i++ {
		fmax += 2 * float64(taps[i+M]) * math.Cos(float64(i)*math.Pi)
	}

	gain /= fmax

	for i := 0; i < nTaps; i++ {
		taps[i] = float32(float64(taps[i]) * gain)
	}

	return taps
}
