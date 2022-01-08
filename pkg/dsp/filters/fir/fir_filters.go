package fir

import (
	"errors"
)

func computeNTapsAtt(sampleRate float64, transitionWidth float64, attenuationDB float64) int {
	ntaps := int(attenuationDB * sampleRate / (22.0 * transitionWidth))
	ntaps |= 1 // make odd

	return ntaps
}

func computeNTaps(sampleRate float64, transitionWidth float64, winType WindowType, param float64) int {
	maxAttenuation := windowMaxAttenuation[winType]
	ntaps := int(
		float64(maxAttenuation) * sampleRate / (22.0 * transitionWidth))
	ntaps |= 1

	return ntaps
}

func tapsForWindowType(winType WindowType, ntaps int, atten int) []float32 {
	switch winType {
	case Hamming:
		return HammingWindow(ntaps)
	case Hann:
		return HannWindow(ntaps)
	case BlackmanHarris:
		return BlackmanHarrisWindow(ntaps, atten)
	default:
		panic(errors.New("unspecified window type"))
	}
}
