package c4fm

import (
	"fmt"
	"math"
	"testing"

	"github.com/mjibson/go-dsp/fft"
)

const ()

func TestGeneration(t *testing.T) {
	NewTapGenerator(
		24000, 4800, 13, 1.0,
	).GenerateTransferRX(1.0)
}

func sliceEqualFloat64(f1, f2 []float64, epsilon float64) bool {
	if len(f1) != len(f2) {
		return false
	}
	for i := 0; i < len(f1); i++ {
		if math.Abs(f1[i]-f2[i]) > epsilon {
			return false
		}
	}
	return true
}

func TestIFFT(t *testing.T) {

	fmt.Println(fft.IFFT([]complex128{1, -1i, -1, 1i}))
	// fmt.Println(fft.I([]complex128{1, -1i, -1}))
}

func TestTransferRX(t *testing.T) {
	if got := transferRX(4800, 1.0); !sliceEqualFloat64(got, generated4800, 1e-8) {
		t.Errorf("TransferRX() = %v (%d), want %v (%d)", got[:16], len(got), generated4800[:16], len(generated4800))
	}
}
