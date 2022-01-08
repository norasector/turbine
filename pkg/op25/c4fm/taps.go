package c4fm

import (
	"fmt"
	"math"
	"sort"

	"github.com/mjibson/go-dsp/fft"
)

const (
	DefaultSymbolRate int     = 3600
	DefaultSpan       int     = 13
	DefaultMultiplier float64 = 1.0
)

type TapGenerator struct {
	sampleRate int
	symbolRate int
	filterGain float64
	sps        int
	ntaps      int
}

func NewTapGenerator(
	sampleRate int,
	symbolRate int,
	span int,
	filterGain float64,
) *TapGenerator {
	ret := &TapGenerator{
		sampleRate: sampleRate,
		symbolRate: symbolRate,
		filterGain: filterGain,
		sps:        sampleRate / symbolRate,
	}

	// ensure that it's an odd number
	ret.ntaps = (ret.sps * span) | 1

	return ret
}

func (g *TapGenerator) GenerateTransferRX(multiplier float64) []float32 {
	taps := transferRX(g.symbolRate, multiplier)

	return g.generate(taps)

}

func getPositiveComponents(c []complex128) []float32 {
	ret := make([]float32, 0, len(c))
	for _, val := range c {
		ival := imag(val)
		if ival > 0 {
			ret = append(ret, float32(ival))
		}
	}
	return ret
}

type ReverseFloat32Slice []float32

func (x ReverseFloat32Slice) Len() int           { return len(x) }
func (x ReverseFloat32Slice) Less(i, j int) bool { return x[i] >= x[j] }
func (x ReverseFloat32Slice) Swap(i, j int)      { x[i], x[j] = x[j], x[i] }

func (g *TapGenerator) generate(generated []float64) []float32 {

	ifft := fft.IFFTReal(generated)
	// baseFFT := fft.FFT
	conj := multiplyByConjugate(ifft)
	fmt.Println("first", conj[:16])
	fmt.Println("last", conj[len(conj)-16:])
	fmt.Println("len", len(conj))

	// impulseResponse := fft.IFFT(ifft)

	// start := argMax(impulseResponse)
	components := ReverseFloat32Slice(getPositiveComponents(ifft))
	sort.Sort(components)
	return components[:16]
}

func transferRX(symbolRate int, multiplier float64) []float64 {
	rate := float64(symbolRate) * multiplier
	ret := make([]float64, int(rate))
	for i := 0; i < int(rate); i++ {
		fi := float64(i)

		t := math.Pi * fi / rate
		if t >= 1e-6 {
			ret[i] = math.Sin(t) / t
		} else {
			ret[i] = 1.0
		}
	}
	return ret
}

// func transferFunctionTX(symbolRate int) []float32 {
// 	ret := make([]float32, 0, 2881)
// 	for i := 0; i < 2881; i++ {
// 		fi := float64(i)
// 		hf := 1.0
// 		if i >= 1920 {
// 			hf = 0.5 + 0.5*math.Cos(2*math.Pi*fi/1920.0)
// 		}

// 		var pf float64 = 1.0

// 		t := math.Pi * fi / 4800.0
// 		if t >= 1e-6 {
// 			pf = t / math.Sin(t)
// 		}
// 		ret = append(ret, float32(pf*hf))
// 	}
// 	return ret
// }
