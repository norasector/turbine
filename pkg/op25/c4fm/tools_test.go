package c4fm

import (
	"reflect"
	"testing"
)

func Test_fftshift(t *testing.T) {
	type args struct {
		freqs []float64
	}
	tests := []struct {
		name string
		args args
		want []float64
	}{{
		"10",
		args{[]float64{0., 1., 2., 3., 4., -5., -4., -3., -2., -1.}},
		[]float64{-5., -4., -3., -2., -1., 0., 1., 2., 3., 4.},
	}, {
		"11",
		args{[]float64{0., 0.90909091, 1.81818182, 2.72727273, 3.63636364,
			4.54545455, -4.54545455, -3.63636364, -2.72727273, -1.81818182,
			-0.90909091}},
		[]float64{-4.54545455, -3.63636364, -2.72727273, -1.81818182, -0.90909091,
			0., 0.90909091, 1.81818182, 2.72727273, 3.63636364,
			4.54545455},
	},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fftshift(tt.args.freqs); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("fftshift() = %v, want %v", got, tt.want)
			}
		})
	}
}
