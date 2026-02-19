package p25

// BCH(63,16,23) codec for P25 NID (Network ID).
// 16 data bits + 47 parity bits = 63 bits, plus 1 overall parity = 64 bits.
// Minimum distance 23, can correct up to 11 bit errors.
// Uses GF(2^6) with primitive polynomial x^6 + x + 1 (same as RS codec).
//
// Codeword format: [data(16) | parity(47)] in the 63-bit BCH word.
// NID format: [BCH_codeword(63) | overall_parity(1)] = 64 bits.
// Data bits: NAC(12) << 4 | DUID(4).
//
// The generator polynomial is computed at init from the GF(2^6) field tables,
// ensuring consistency between encoder and decoder.

// bchGenPoly is the generator polynomial for BCH(63,16,23), degree 47.
// Bit i is the coefficient of x^i. Computed at init time.
var bchGenPoly uint64

func init() {
	// Ensure GF(2^6) tables are initialized before computing the generator polynomial.
	// (bch.go sorts before rs.go, so rs.go's init may not have run yet.)
	initGaloisField()
	bchGenPoly = computeBCHGenPoly()
}

// computeBCHGenPoly derives the BCH(63,16,23) generator polynomial from
// the GF(2^6) field. It is the LCM of the minimal polynomials of
// alpha^1, alpha^2, ..., alpha^22.
func computeBCHGenPoly() uint64 {
	visited := make(map[int]bool)
	var g uint64 = 1 // start with g(x) = 1

	for i := 1; i <= 22; i++ {
		if visited[i] {
			continue
		}

		// Find cyclotomic coset of i: {i, 2i, 4i, 8i, 16i, 32i} mod 63
		coset := cyclotomicCoset(i)
		for _, j := range coset {
			visited[j] = true
		}

		// Compute minimal polynomial of alpha^i over GF(2)
		mp := minimalPoly(coset)

		// Multiply g(x) by the minimal polynomial
		g = gf2PolyMul(g, mp)
	}

	return g
}

// cyclotomicCoset returns the cyclotomic coset of i modulo 63.
// C(i) = {i, 2i, 4i, 8i, 16i, 32i} mod 63 (deduplicated).
func cyclotomicCoset(i int) []int {
	var coset []int
	seen := make(map[int]bool)
	val := i % 63
	for {
		if seen[val] {
			break
		}
		seen[val] = true
		coset = append(coset, val)
		val = (val * 2) % 63
	}
	return coset
}

// minimalPoly computes the minimal polynomial of alpha^(coset[0]) over GF(2).
// The result is a polynomial with binary coefficients, represented as a uint64.
// m(x) = product of (x + alpha^j) for j in coset, computed in GF(2^6).
func minimalPoly(coset []int) uint64 {
	// Polynomial coefficients in GF(2^6).
	// poly[k] = coefficient of x^k.
	// Start with poly = 1.
	poly := make([]uint8, 1)
	poly[0] = 1

	for _, j := range coset {
		aj := gfExp[j%63] // alpha^j
		// Multiply poly by (x + alpha^j)
		newPoly := make([]uint8, len(poly)+1)
		for k := len(poly) - 1; k >= 0; k-- {
			newPoly[k+1] ^= poly[k]             // x * poly[k]
			newPoly[k] ^= gfMul(poly[k], aj)    // alpha^j * poly[k]
		}
		poly = newPoly
	}

	// Convert GF(2^6) coefficients to binary (they should all be 0 or 1)
	var result uint64
	for k := 0; k < len(poly); k++ {
		if poly[k] != 0 {
			result |= 1 << uint(k)
		}
	}
	return result
}

// gf2PolyMul multiplies two polynomials over GF(2) represented as uint64 bitmasks.
func gf2PolyMul(a, b uint64) uint64 {
	var result uint64
	for b != 0 {
		if b&1 != 0 {
			result ^= a
		}
		a <<= 1
		b >>= 1
	}
	return result
}

// BCHEncode63 encodes 16 data bits into a 63-bit BCH(63,16,23) codeword.
// Systematic encoding: data at bits 62-47, parity at bits 46-0.
func BCHEncode63(data uint16) uint64 {
	// Shift data into the high 16 bits of the 63-bit word
	shifted := uint64(data) << 47

	// Polynomial long division: compute shifted mod g(x)
	remainder := shifted
	for i := 62; i >= 47; i-- {
		if remainder&(1<<uint(i)) != 0 {
			remainder ^= bchGenPoly << uint(i-47)
		}
	}

	// Systematic codeword = original data position | remainder
	return (uint64(data) << 47) | (remainder & ((1 << 47) - 1))
}

// BCHEncodeNID creates a 64-bit P25 NID from NAC and DUID.
// Returns [BCH_codeword(63) | overall_parity(1)].
func BCHEncodeNID(nac uint16, duid DUID) uint64 {
	infoBits := (nac&0xFFF)<<4 | uint16(duid&0x0F)
	cw := BCHEncode63(infoBits)
	p := popcount64(cw) & 1
	return (cw << 1) | p
}

// BCHDecode63 decodes a 63-bit BCH(63,16,23) codeword.
// Returns (decoded 16-bit data, error count, valid).
func BCHDecode63(received uint64) (uint16, int, bool) {
	received &= (1 << 63) - 1 // mask to 63 bits

	// Compute syndromes S_1 through S_22
	var syndromes [23]uint8 // syndromes[j] for j=1..22
	allZero := true
	for j := 1; j <= 22; j++ {
		s := bchSyndrome(received, j)
		syndromes[j] = s
		if s != 0 {
			allZero = false
		}
	}

	if allZero {
		return uint16(received >> 47), 0, true
	}

	// Berlekamp-Massey to find error locator polynomial
	sigma, L := bchBerlekampMassey(syndromes)

	// Chien search for error positions
	errorPositions := bchChienSearch(sigma, L)
	if errorPositions == nil {
		return uint16(received >> 47), -1, false
	}

	// Correct errors (binary BCH: just flip bits)
	corrected := received
	for _, pos := range errorPositions {
		corrected ^= 1 << uint(pos)
	}

	return uint16(corrected >> 47), len(errorPositions), true
}

// BCHDecodeNID decodes a 64-bit P25 NID.
// Returns (NAC, DUID, error count, valid).
func BCHDecodeNID(nid uint64) (uint16, DUID, int, bool) {
	// Strip overall parity bit (bit 0)
	codeword := nid >> 1

	data, nerrs, valid := BCHDecode63(codeword)
	if !valid {
		return 0, 0, nerrs, false
	}

	nac := (data >> 4) & 0xFFF
	duid := DUID(data & 0x0F)
	return nac, duid, nerrs, valid
}

// bchSyndrome computes syndrome S_j = r(alpha^j) for the received word.
func bchSyndrome(word uint64, j int) uint8 {
	var s uint8
	for i := 0; i < 63; i++ {
		if word&(1<<uint(i)) != 0 {
			exp := (i * j) % 63
			s ^= gfExp[exp]
		}
	}
	return s
}

// bchBerlekampMassey runs the Berlekamp-Massey algorithm over GF(2^6)
// to find the error locator polynomial from the syndromes.
func bchBerlekampMassey(syndromes [23]uint8) ([]uint8, int) {
	const n = 22

	// Error locator polynomial sigma(x) = sigma[0] + sigma[1]*x + ...
	sigma := make([]uint8, n+1)
	sigma[0] = 1

	prev := make([]uint8, n+1)
	prev[0] = 1

	L := 0
	m := 1
	b := uint8(1)

	for r := 1; r <= n; r++ {
		// Compute discrepancy d = S_r + sum(sigma[i] * S_{r-i}) for i=1..L
		d := syndromes[r]
		for i := 1; i <= L; i++ {
			if sigma[i] != 0 && syndromes[r-i] != 0 {
				d ^= gfMul(sigma[i], syndromes[r-i])
			}
		}

		if d == 0 {
			m++
			continue
		}

		// Save current sigma
		temp := make([]uint8, n+1)
		copy(temp, sigma)

		// sigma(x) += (d/b) * x^m * prev(x)
		factor := gfMul(d, gfInv(b))
		for i := m; i <= n; i++ {
			if prev[i-m] != 0 {
				sigma[i] ^= gfMul(factor, prev[i-m])
			}
		}

		if 2*L <= r-1 {
			L = r - L
			copy(prev, temp)
			b = d
			m = 1
		} else {
			m++
		}
	}

	return sigma, L
}

// bchChienSearch finds the roots of the error locator polynomial sigma(x)
// by evaluating at all elements alpha^(-i) for i=0..62.
// Returns error positions, or nil if the number of roots doesn't match L.
func bchChienSearch(sigma []uint8, L int) []int {
	var errors []int

	for pos := 0; pos < 63; pos++ {
		// Evaluate sigma(alpha^{-pos}) = sigma(alpha^{(63-pos) mod 63})
		inv := (63 - pos) % 63
		val := uint8(0)
		for j := 0; j <= L; j++ {
			if sigma[j] != 0 {
				exp := (j * inv) % 63
				val ^= gfMul(sigma[j], gfExp[exp])
			}
		}
		if val == 0 {
			errors = append(errors, pos)
		}
	}

	if len(errors) != L {
		return nil
	}

	return errors
}

// popcount64 returns the number of set bits in x.
func popcount64(x uint64) uint64 {
	x = x - ((x >> 1) & 0x5555555555555555)
	x = (x & 0x3333333333333333) + ((x >> 2) & 0x3333333333333333)
	x = (x + (x >> 4)) & 0x0F0F0F0F0F0F0F0F
	return (x * 0x0101010101010101) >> 56
}
