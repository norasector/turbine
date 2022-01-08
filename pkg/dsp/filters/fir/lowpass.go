package fir

import (
	"math"
)

func MakeLowPass2(gain, sampleRate, cutFrequency, transitionWidth, attenuation float64, winType WindowType) []float32 {
	var nTaps = computeNTapsAtt(sampleRate, transitionWidth, attenuation)
	var taps = make([]float32, nTaps)

	var w = tapsForWindowType(winType, nTaps, int(attenuation))

	var M = (nTaps - 1) / 2
	var fwT0 = 2 * math.Pi * cutFrequency / sampleRate

	for i := -M; i <= M; i++ {
		if i == 0 {
			taps[i+M] = float32(fwT0) / math.Pi * w[i+M]
		} else {
			taps[i+M] = float32(math.Sin(float64(i)*fwT0) / (float64(i) * math.Pi) * float64(w[i+M]))
		}
	}

	var fmax = float64(taps[M])
	for i := 1; i <= M; i++ {
		fmax += 2 * float64(taps[i+M])
	}

	gain /= fmax

	for i := 0; i < nTaps; i++ {
		taps[i] = float32(float64(taps[i]) * gain)
	}

	return taps
}

func MakeLowPass(gain, sampleRate, cutFrequency, transitionWidth float64, winType WindowType) []float32 {
	nTaps := computeNTaps(sampleRate, transitionWidth, winType, 0.)
	var taps = make([]float32, nTaps)
	var w = windowFuncs[winType](nTaps)

	var M = (nTaps - 1) / 2
	var fwT0 = 2 * math.Pi * cutFrequency / sampleRate

	for i := -M; i <= M; i++ {
		if i == 0 {
			taps[i+M] = float32(fwT0 / math.Pi * float64(w[i+M]))
		} else {
			fi := float64(i)
			taps[i+M] = float32(math.Sin(fi*fwT0) / (fi * math.Pi) * float64(w[i+M]))
		}
	}

	var fmax = float64(taps[0+M])
	for i := 1; i <= M; i++ {
		fmax += 2 * float64(taps[i+M])
	}

	gain /= fmax

	for i := 0; i < nTaps; i++ {
		taps[i] = float32(float64(taps[i]) * gain)
	}

	return taps
}
