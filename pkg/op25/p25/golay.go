package p25

// Extended Golay (24,12,8) codec.
// 12 data bits + 12 parity bits = 24 bits.
// Minimum distance 8, can correct up to 3 bit errors.
// Used for P25 NID decoding.
//
// Codeword format: [data(12) | parity(12)] (MSB first)
// Parity = [11-bit Golay remainder | 1-bit overall parity]
//
// Generator polynomial for (23,12) Golay code:
//   g(x) = x^11 + x^10 + x^6 + x^5 + x^4 + x^2 + 1 = 0xC75

const golayGen = 0xC75 // generator polynomial

// golayB is the 12x12 parity generation matrix.
// Computed from the generator polynomial at init time.
// B[i] = parity for data with a single 1 at position (11-i) from LSB.
var golayB [12]uint32

// golayBT is the transpose of golayB.
// Needed for syndrome decoding: since the extended Golay code is self-dual,
// B^(-1) = B^T, so we use B^T to invert the syndrome in steps 3-4.
var golayBT [12]uint32

func init() {
	golayB = computeGolayB()
	golayBT = transposeB(golayB)
}

// transposeB computes the transpose of a 12x12 binary matrix.
func transposeB(B [12]uint32) [12]uint32 {
	var BT [12]uint32
	for i := 0; i < 12; i++ {
		for j := 0; j < 12; j++ {
			if B[i]&(1<<uint(11-j)) != 0 {
				BT[j] |= 1 << uint(11-i)
			}
		}
	}
	return BT
}

// computeGolayB derives the B matrix from the generator polynomial.
// For each data bit position i, B[i] = parity bits for a unit data vector.
func computeGolayB() [12]uint32 {
	var B [12]uint32
	for i := 0; i < 12; i++ {
		// Compute x^(22-i) mod g(x) by repeated multiply-by-x
		r := uint32(1)
		for j := 0; j < 22-i; j++ {
			r <<= 1
			if r&(1<<11) != 0 {
				r ^= golayGen
			}
		}
		remainder := r & 0x7FF // 11-bit Golay remainder

		// Overall parity bit: make total weight even
		// Unit data vector has weight 1, remainder has some weight
		totalWeight := 1 + weight11(remainder)
		overallParity := uint32(totalWeight & 1) // 1 if odd (need to make even)

		B[i] = (remainder << 1) | overallParity
	}
	return B
}

func weight11(x uint32) int {
	x &= 0x7FF
	w := 0
	for x != 0 {
		w += int(x & 1)
		x >>= 1
	}
	return w
}

// weight12 returns the Hamming weight of a 12-bit value.
func weight12(x uint32) int {
	x &= 0xFFF
	w := 0
	for x != 0 {
		w += int(x & 1)
		x >>= 1
	}
	return w
}

// golayMultB multiplies a 12-bit vector by the B matrix, returning 12-bit result.
func golayMultB(v uint32) uint32 {
	v &= 0xFFF
	var result uint32
	for i := 0; i < 12; i++ {
		if v&(1<<uint(11-i)) != 0 {
			result ^= golayB[i]
		}
	}
	return result & 0xFFF
}

// golayMultBT multiplies a 12-bit vector by B^T (transpose of B).
func golayMultBT(v uint32) uint32 {
	v &= 0xFFF
	var result uint32
	for i := 0; i < 12; i++ {
		if v&(1<<uint(11-i)) != 0 {
			result ^= golayBT[i]
		}
	}
	return result & 0xFFF
}

// GolayEncode24 encodes 12 data bits into a 24-bit extended Golay codeword.
// Format: [data(12) | parity(12)]
func GolayEncode24(data uint32) uint32 {
	data &= 0xFFF
	parity := golayMultB(data)
	return (data << 12) | parity
}

// GolayDecode24 decodes a 24-bit extended Golay codeword.
// Returns (decoded 12-bit data, number of corrected errors, valid).
// Can correct up to 3 bit errors.
//
// Algorithm (syndrome decoding for extended Golay code):
// 1. Compute syndrome s = received_parity XOR expected_parity
// 2. If weight(s) <= 3: error is in parity only → no data correction
// 3. For each row B[i]: if weight(s ^ B[i]) <= 2 → flip data bit i
// 4. Compute s' = B^T * s; if weight(s') <= 3 → error is in data: data ^= s'
// 5. For each row BT[i]: if weight(s' ^ BT[i]) <= 2 → flip parity bit i, data ^= (s'^BT[i])
// 6. Otherwise uncorrectable
func GolayDecode24(codeword uint32) (uint32, int, bool) {
	codeword &= 0xFFFFFF
	data := (codeword >> 12) & 0xFFF
	parity := codeword & 0xFFF

	// Compute syndrome
	s := parity ^ golayMultB(data)

	// Step 1: weight(s) <= 3 → error only in parity bits
	w := weight12(s)
	if w <= 3 {
		return data, w, true
	}

	// Step 2: try s ^ B[i] for each data bit position
	for i := 0; i < 12; i++ {
		t := s ^ golayB[i]
		wt := weight12(t)
		if wt <= 2 {
			data ^= 1 << uint(11-i)
			return data, wt + 1, true
		}
	}

	// Step 3: compute s' = B^T * s (using B^T = B^(-1) for self-dual code)
	sp := golayMultBT(s)
	w = weight12(sp)
	if w <= 3 {
		// Error entirely in data half
		data ^= sp
		return data, w, true
	}

	// Step 4: try s' ^ BT[i] for each parity bit position
	for i := 0; i < 12; i++ {
		t := sp ^ golayBT[i]
		wt := weight12(t)
		if wt <= 2 {
			// Error: t in data, plus bit i in parity
			data ^= t
			return data, wt + 1, true
		}
	}

	// Uncorrectable (4+ bit errors)
	return data, -1, false
}
