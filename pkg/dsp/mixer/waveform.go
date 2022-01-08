package mixer

import (
	"math"
)

const (
	tau float64 = math.Pi * 2
)

type WaveformMixer struct {
	sampleRate     int
	frequency      int
	phase          float64
	phaseIncrement float64
}

func (w *WaveformMixer) incrementPhase() {
	w.phase += w.phaseIncrement
	if w.phase > tau {
		w.phase -= tau
	} else if w.phase < -tau {
		w.phase += tau
	}
}

func NewWaveformMixer(sampleRate int, frequency int) *WaveformMixer {
	ret := &WaveformMixer{
		sampleRate:     sampleRate,
		frequency:      frequency,
		phaseIncrement: float64(frequency) * tau / float64(sampleRate),
		phase:          0.0,
	}

	return ret
}

func (w *WaveformMixer) WorkBuffer(input []complex64, output []complex64) int {

	for i := 0; i < len(input); i++ {

		sin, cos := math.Sincos(w.phase)

		output[i] = complex(float32(cos), float32(sin)) * input[i]
		w.incrementPhase()
	}

	return len(input)

}

func (w *WaveformMixer) Work(vals []complex64) []complex64 {

	ret := make([]complex64, len(vals))
	w.WorkBuffer(vals, ret)
	return ret

}

func (w *WaveformMixer) PredictOutputSize(inputSize int) int {
	return inputSize
}
