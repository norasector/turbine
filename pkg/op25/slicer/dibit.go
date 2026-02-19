package slicer

// DibitSlicer maps FSK4 demodulator output levels to 2-bit dibit values.
// It implements the FBWorker interface (float32 in, byte out).
//
// Mapping (based on symbol spread from FSK4 demodulator):
//   input >= +2.0 → 0 (dibit 00, symbol +3)
//   input >= 0.0  → 1 (dibit 01, symbol +1)
//   input >= -2.0 → 2 (dibit 10, symbol -1)
//   else          → 3 (dibit 11, symbol -3)
//
// Output: one byte per dibit (values 0-3), not bit-packed.
type DibitSlicer struct{}

func NewDibitSlicer() *DibitSlicer {
	return &DibitSlicer{}
}

func sliceDibit(f float32) byte {
	if f >= 2.0 {
		return 0
	} else if f >= 0.0 {
		return 1
	} else if f >= -2.0 {
		return 2
	}
	return 3
}

func (d *DibitSlicer) WorkBuffer(input []float32, output []byte) int {
	for i := 0; i < len(input); i++ {
		output[i] = sliceDibit(input[i])
	}
	return len(input)
}

func (d *DibitSlicer) Work(items []float32) []byte {
	ret := make([]byte, len(items))
	d.WorkBuffer(items, ret)
	return ret
}

func (d *DibitSlicer) PredictOutputSize(inputSize int) int {
	return inputSize
}
