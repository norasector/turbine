package slicer

// BinarySlicer takes input float32 data and returns a byte
// with value 0 or 1 depending on the sign of the value.
type BinarySlicer struct {
	invert bool
}

func NewBinarySlicer(invert bool) *BinarySlicer {
	return &BinarySlicer{
		invert: invert,
	}
}

// "why would you want to invert this?"
// because you have a bug that's causing sign inversion
// and this was faster than tracking that bug down
func slice(f float32, invert bool) byte {
	if invert {
		if f >= 0 {
			return 0 //1
		}
		return 1 //0
	} else {
		if f >= 0 {
			return 1
		}
		return 0
	}
}

func (b *BinarySlicer) WorkBuffer(input []float32, output []byte) int {
	for i := 0; i < len(input); i++ {
		output[i] = slice(input[i], b.invert)

	}
	return len(input)
}

func (b *BinarySlicer) Work(items []float32) []byte {
	ret := make([]byte, len(items))
	b.WorkBuffer(items, ret)
	return ret
}

func (b *BinarySlicer) PredictOutputSize(inputSize int) int {
	return inputSize
}
