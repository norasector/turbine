package rmsagc

import (
	"math"
)

// RMSAGC is a root-mean-squared automatic gain controller
type RMSAGC struct {
	alpha   float64
	beta    float64
	gain    float64
	average float64
}

func NewRMSAGC(alpha float64, k float64) *RMSAGC {
	return &RMSAGC{
		alpha:   alpha,
		beta:    1 - alpha,
		average: 1.0,
		gain:    k,
	}
}

func (r *RMSAGC) PredictOutputSize(inputSize int) int {
	return inputSize
}

func (r *RMSAGC) WorkBuffer(input, output []float32) int {
	for i := 0; i < len(input); i++ {
		cur := float64(input[i])
		magSquared := cur * cur
		r.average = r.beta*r.average + r.alpha*magSquared
		if r.average > 0 {
			output[i] = float32(r.gain * cur / math.Sqrt(r.average))
		} else {
			output[i] = float32(r.gain * cur)
		}
	}

	return len(input)
}

func (r *RMSAGC) Work(data []float32) []float32 {
	ret := make([]float32, len(data))
	r.WorkBuffer(data, ret)
	return ret
}
