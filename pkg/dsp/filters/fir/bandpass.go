package fir

import "math"

func MakeBandPass(gain, sampleRate, lowCut, highCut, transitionWidth float64, winType WindowType) []float32 {
	var nTaps = computeNTaps(sampleRate, transitionWidth, winType, 0.)
	winFunc := windowFuncs[winType]
	var taps = make([]float32, nTaps)
	var w = winFunc(nTaps)

	var M = (nTaps - 1) / 2

	var fwT0 = 2 * math.Pi * lowCut / sampleRate
	var fwT1 = 2 * math.Pi * highCut / sampleRate

	for i := -M; i <= M; i++ {
		fi := float64(i)
		if i == 0 {
			taps[i+M] = float32((fwT1 - fwT0) / math.Pi * float64(w[i+M]))
		} else {
			taps[i+M] = float32(
				(math.Sin(fi*fwT1) - math.Sin(fi*fwT0)) /
					(float64(i) * math.Pi) *
					float64(w[i+M]),
			)
		}
	}

	var fmax = float64(taps[0+M])
	for i := 1; i <= M; i++ {
		fi := float64(i)
		fmax += 2 * float64(taps[i+M]) * math.Cos(fi*(fwT0+fwT1)*0.5)
	}

	gain /= fmax

	for i := 0; i < nTaps; i++ {
		taps[i] = float32(float64(taps[i]) * gain)
	}

	return taps
}

func MakeComplexBandPass(gain,
	sampleRate,
	lowCut,
	highCut,
	transitionWidth float64,
	winType WindowType) []complex64 {

	ntaps := computeNTaps(sampleRate, transitionWidth, winType, 0.0)

	ret := make([]complex64, ntaps)

	lptaps := MakeLowPass(
		gain,
		sampleRate,
		(highCut-lowCut)/2.0,
		transitionWidth,
		winType)

	freq := math.Pi * (highCut + lowCut) / sampleRate
	var phase float64
	if len(lptaps)&01 > 0 {

		phase = -freq * float64(len(lptaps)>>1)
	} else {
		phase = -freq / 2.0 * float64((2*len(lptaps)+1)>>1)
	}

	for i := 0; i < len(lptaps); i++ {
		curLPTap := float64(lptaps[i])

		ret[i] = complex(
			float32(curLPTap*math.Cos(phase)),
			float32(curLPTap*math.Sin(phase)))

		phase += freq
	}

	return ret
}
