package quad

import (
	"math"

	"github.com/racerxdl/segdsp/dsp"
)

type QuadDemod struct {
	gain    float32
	history []complex64
}

func MakeQuadDemod(gain float32) *QuadDemod {
	return &QuadDemod{
		gain:    gain,
		history: make([]complex64, 2),
	}
}

func (f *QuadDemod) Work(data []complex64) []float32 {
	out := make([]float32, f.PredictOutputSize(len(data)))

	f.WorkBuffer(data, out)

	return out
}

func (f *QuadDemod) WorkBuffer(input []complex64, output []float32) int {
	var samples = append(f.history, input...)
	var tmp = dsp.MultiplyConjugate(samples[1:], samples, len(input))

	for i := 0; i < len(input); i++ {
		output[i] = f.gain * float32(math.Atan2(float64(imag(tmp[i])), float64(real(tmp[i]))))
	}

	f.history = samples[len(input):]
	return len(input)
}

func (f *QuadDemod) PredictOutputSize(inputLength int) int {
	return inputLength
}
