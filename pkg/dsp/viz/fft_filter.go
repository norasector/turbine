package viz

import (
	"sort"

	dspfft "github.com/mjibson/go-dsp/fft"
)

type FFTFilter struct {
	f64Buf           []float64
	FFTPlotterInput  *FFTPlotter
	FFTPlotterOutput *FFTPlotter
}

func NewFFTFilter(
	sampleRate int,
	taps []float32,
) *FFTFilter {
	ret := &FFTFilter{}

	return ret
}

func nextRadix(size int) int {
	radix := 16

	for {
		if size > radix {
			radix *= 2
		} else {
			return radix
		}
	}
}

type indexedSample struct {
	index int
	value complex128
}

type indexedSamples []indexedSample

func (i indexedSamples) Len() int {
	return len(i)
}

// sort in reverse order
func (is indexedSamples) Less(i, j int) bool {
	return real(is[i].value) > real(is[j].value)
}

func (is indexedSamples) Swap(i, j int) {
	is[i], is[j] = is[j], is[i]
}

func findPeaks(fft []complex128, numPeaks int) []int {
	windowSize := 13
	peaks := make(indexedSamples, 0)
	for i := 0; i < len(fft)-windowSize; i++ {
		max := real(fft[i])
		maxIdx := 0
		for j := i + 1; j < windowSize+i; j++ {
			fjr := real(fft[j])
			if fjr > max {
				maxIdx = j - i
				max = fjr
			}
		}

		if maxIdx == windowSize/2 {
			peaks = append(peaks, indexedSample{index: maxIdx + i, value: complex(max, 0)})
		}
	}

	sort.Sort(peaks)

	ret := make([]int, numPeaks)
	cnt := 0
	for i := 0; i < numPeaks && i < len(peaks); i++ {
		ret[i] = peaks[i].index
		cnt++
	}

	return ret[:cnt]
}

func (f *FFTFilter) WorkBuffer(input, output []float32) int {
	fftSize := nextRadix(len(input))
	if len(f.f64Buf) != fftSize {
		f.f64Buf = make([]float64, fftSize)
	}
	allZero := true
	for i := 0; i < len(input); i++ {
		f.f64Buf[i] = float64(input[i])
		if input[i] != 0 {
			allZero = false
		}
	}
	if f.FFTPlotterInput != nil {
		f.FFTPlotterInput.AppendFloat(input)
	}
	if allZero {
		copy(input, output)
		return len(input)
	}
	result := dspfft.FFTReal(f.f64Buf)

	numPeaks := 11
	peaks := findPeaks(result, numPeaks)
	for _, peak := range peaks {

		start := peak - 5
		if start < 0 {
			start = 0
		}
		end := peak + 5
		if end > len(result)-1 {
			end = len(result) - 1
		}

		for i := start; i < end; i++ {
			result[i] = complex(real(result[i])*0.85, 0)
		}
	}

	for i := int(float64(len(result)) / 24 * 2.7); i < len(result); i++ {
		result[i] = complex(0, 0)
	}

	inverse := dspfft.IFFT(result)

	for i := 0; i < len(input); i++ {
		output[i] = float32(real(inverse[i]))
	}

	if f.FFTPlotterOutput != nil {
		f.FFTPlotterOutput.AppendFloat(output[:len(input)])
	}

	return len(input)
}

func (f *FFTFilter) PredictOutputSize(inputSize int) int {
	return inputSize
}
