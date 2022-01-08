package c4fm

func Float32SliceToFloat64(f []float32) []float64 {
	ret := make([]float64, len(f))
	for i := 0; i < len(f); i++ {
		ret[i] = float64(f[i])
	}
	return ret
}

func fftshift(freqs []float64) []float64 {
	midpoint := len(freqs) / 2
	if len(freqs)%2 == 0 {
		midpoint--
	}

	ret := make([]float64, 0, len(freqs))
	ret = append(ret, freqs[midpoint+1:]...)
	ret = append(ret, freqs[0:midpoint+1]...)
	return ret
}

func multiplyByConjugate(f []complex128) []float64 {
	ret := make([]float64, len(f))
	for i := 0; i < len(f); i++ {
		rn, in := real(f[i]), imag(f[i])
		if rn > 0 {
			ret[i] = rn
		} else {
			ret[i] = (rn * rn) + (in * in)
		}
		// ret[i] = (rn * rn) + (in * in)
	}
	return ret
}
