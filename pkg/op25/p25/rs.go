package p25

// Reed-Solomon decoders for P25.
// P25 uses RS codes over GF(2^6) = GF(64).
//
// RS(24,12,13) — used for link control in LDU1
// RS(24,16,9)  — used for encryption sync in LDU2
//
// Primitive polynomial for GF(64): x^6 + x + 1 (0x43)

const (
	gfBits = 6
	gfSize = 63 // 2^6 - 1
	gfPoly = 0x43
)

var (
	gfExp [128]uint8 // exp table (alpha^i)
	gfLog [64]uint8  // log table (log_alpha(x))
)

func init() {
	initGaloisField()
}

func initGaloisField() {
	var x uint8 = 1
	for i := 0; i < gfSize; i++ {
		gfExp[i] = x
		gfLog[x] = uint8(i)
		x <<= 1
		if x&0x40 != 0 { // x >= 64
			x ^= gfPoly
		}
		x &= 0x3F
	}
	gfExp[gfSize] = gfExp[0]
	// Extend exp table for convenience
	for i := gfSize + 1; i < 128; i++ {
		gfExp[i] = gfExp[i-gfSize]
	}
}

func gfMul(a, b uint8) uint8 {
	if a == 0 || b == 0 {
		return 0
	}
	return gfExp[(int(gfLog[a])+int(gfLog[b]))%gfSize]
}

func gfInv(a uint8) uint8 {
	if a == 0 {
		return 0
	}
	return gfExp[gfSize-int(gfLog[a])]
}

func gfAdd(a, b uint8) uint8 {
	return a ^ b
}

// RSDecode2412 performs Reed-Solomon (24,12,13) decoding.
// Input: 24 six-bit symbols. First 12 are data, last 12 are parity.
// Returns: (corrected data symbols, success).
// Can correct up to 6 symbol errors.
func RSDecode2412(symbols [24]uint8) ([12]uint8, bool) {
	corrected, ok := rsDecode(symbols[:], 24, 12, 12)
	var result [12]uint8
	copy(result[:], corrected)
	return result, ok
}

// RSDecode2416 performs Reed-Solomon (24,16,9) decoding.
// Input: 24 six-bit symbols. First 16 are data, last 8 are parity.
// Returns: (corrected data symbols, success).
// Can correct up to 4 symbol errors.
func RSDecode2416(symbols [24]uint8) ([16]uint8, bool) {
	corrected, ok := rsDecode(symbols[:], 24, 16, 8)
	var result [16]uint8
	copy(result[:], corrected)
	return result, ok
}

// rsDecode is a generic RS decoder using the Berlekamp-Massey algorithm.
// Returns (corrected data symbols as slice, success).
func rsDecode(received []uint8, n, k, npar int) ([]uint8, bool) {
	result := make([]uint8, k)

	// Calculate syndromes
	syndromes := make([]uint8, npar)
	allZero := true
	for i := 0; i < npar; i++ {
		var s uint8
		for j := 0; j < n; j++ {
			s = gfAdd(gfMul(s, gfExp[i+1]), received[j]&0x3F)
		}
		syndromes[i] = s
		if s != 0 {
			allZero = false
		}
	}

	if allZero {
		copy(result, received[:k])
		return result, true
	}

	// Berlekamp-Massey algorithm
	t := npar / 2
	errLoc := make([]uint8, t+1)
	errLoc[0] = 1

	oldLoc := make([]uint8, t+1)
	oldLoc[0] = 1

	var l int
	for i := 0; i < npar; i++ {
		var delta uint8
		for j := 0; j <= l; j++ {
			delta = gfAdd(delta, gfMul(errLoc[j], syndromes[i-j]))
		}
		oldLoc = append([]uint8{0}, oldLoc...)
		if len(oldLoc) > t+1 {
			oldLoc = oldLoc[:t+1]
		}

		if delta != 0 {
			if 2*l <= i {
				newLoc := make([]uint8, len(errLoc))
				copy(newLoc, errLoc)
				scale := gfMul(delta, gfInv(oldLoc[0]))
				for j := 0; j < len(oldLoc) && j < len(errLoc); j++ {
					errLoc[j] = gfAdd(errLoc[j], gfMul(scale, oldLoc[j]))
				}
				l = i + 1 - l
				oldLoc = newLoc
				oldLoc[0] = gfMul(oldLoc[0], gfInv(delta))
			} else {
				scale := delta
				for j := 0; j < len(oldLoc) && j < len(errLoc); j++ {
					errLoc[j] = gfAdd(errLoc[j], gfMul(scale, oldLoc[j]))
				}
			}
		}
	}

	// Find error positions using Chien search
	errCount := l
	if errCount > t {
		copy(result, received[:k])
		return result, false
	}

	errPos := make([]int, 0, errCount)
	for i := 0; i < n; i++ {
		var sum uint8
		for j := 0; j <= errCount; j++ {
			sum = gfAdd(sum, gfMul(errLoc[j], gfExp[(j*i)%gfSize]))
		}
		if sum == 0 {
			errPos = append(errPos, n-1-i)
		}
	}

	if len(errPos) != errCount {
		copy(result, received[:k])
		return result, false
	}

	// Forney algorithm to compute error values
	corrected := make([]uint8, n)
	copy(corrected, received)
	for _, pos := range errPos {
		xi := gfExp[(gfSize-pos)%gfSize]

		// Evaluate error evaluator polynomial
		var num uint8
		xiPow := uint8(1)
		for j := 0; j < npar; j++ {
			num = gfAdd(num, gfMul(syndromes[j], xiPow))
			xiPow = gfMul(xiPow, xi)
		}

		// Evaluate derivative of error locator
		var den uint8
		for _, p := range errPos {
			if p != pos {
				den = gfAdd(den, gfMul(gfExp[(gfSize-p)%gfSize], xi))
			}
		}
		if den == 0 {
			den = 1
		}

		corrected[pos] = gfAdd(corrected[pos], gfMul(num, gfInv(den)))
	}

	copy(result, corrected[:k])
	return result, true
}
