package fir

import (
	"errors"
	"math"
)

type WindowFunc func(int) []float32

type WindowType int

const (
	Hamming        WindowType = 0
	Hann           WindowType = 1
	BlackmanHarris WindowType = 2
	Blackman       WindowType = 3
)

var (
	windowMaxAttenuation = map[WindowType]int{
		Hamming:        53,
		Hann:           44,
		BlackmanHarris: 92,
		Blackman:       74,
	}
	windowFuncs = map[WindowType]WindowFunc{
		Hamming:  HammingWindow,
		Hann:     HannWindow,
		Blackman: BlackmanWindow,
		// BlackmanHarris: BlackmanHarrisWindow,
	}
)

func cosWindow1(ntaps int, c0, c1, c2 float64) []float32 {
	ret := make([]float32, ntaps)
	M := float64(ntaps - 1)

	for i := 0; i < ntaps; i++ {
		fi := float64(i)
		ret[i] = float32(c0 - c1*math.Cos((2*math.Pi*fi)/M) +
			c2*math.Cos((4*math.Pi*fi)/M))
	}
	return ret
}

func cosWindow2(ntaps int, c0, c1, c2, c3 float64) []float32 {
	ret := make([]float32, ntaps)
	M := float64(ntaps - 1)

	for i := 0; i < ntaps; i++ {
		fi := float64(i)
		ret[i] = float32(c0 - c1*math.Cos((2*math.Pi*fi)/M) +
			c2*math.Cos((4*math.Pi*fi)/M) -
			c3*math.Cos((6*math.Pi*fi)/M) +
			c3*math.Cos((8*math.Pi*fi)/M))
	}
	return ret
}

func cosWindow3(ntaps int, c0, c1, c2, c3, c4 float64) []float32 {
	ret := make([]float32, ntaps)
	M := float64(ntaps - 1)

	for i := 0; i < ntaps; i++ {
		fi := float64(i)
		ret[i] = float32(c0 - c1*math.Cos((2*math.Pi*fi)/M) +
			c2*math.Cos((4*math.Pi*fi)/M) -
			c3*math.Cos((6*math.Pi*fi)/M))
	}
	return ret
}

func BlackmanHarrisWindow(ntaps, atten int) []float32 {
	switch atten {
	case 61:
		return cosWindow1(ntaps, 0.42323, 0.49755, 0.07922)
	case 67:
		return cosWindow1(ntaps, 0.44959, 0.49364, 0.05677)
	case 74:
		return cosWindow2(ntaps, 0.40271, 0.49703, 0.09392, 0.00183)
	case 92:
		return cosWindow2(ntaps, 0.35875, 0.48829, 0.14128, 0.01168)
	default:
		panic(errors.New("blackman harris window must have attenuation value 61, 67, 74, 92"))

	}
}

func BlackmanWindow(ntaps int) []float32 {
	return cosWindow1(ntaps, 0.42, 0.5, 0.08)
}

func HammingWindow(ntaps int) []float32 {
	ret := make([]float32, ntaps)
	M := float64(ntaps - 1)

	for i := 0; i < ntaps; i++ {
		ret[i] = float32(0.54 - 0.46*math.Cos((2.0*math.Pi*float64(i))/M))
	}

	return ret
}

func HannWindow(taps int) []float32 {
	ret := make([]float32, taps)
	M := float64(taps - 1)
	for i := 0; i < taps; i++ {
		cosVal := 2 * math.Pi * float64(i)
		ret[i] = float32(0.5 - 0.5*math.Cos(cosVal/M))
	}
	return ret
}
