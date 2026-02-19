package p25

// Hamming decoders for P25 link control fields.

// Hamming1511Decode decodes a (15,11,3) Hamming codeword.
// Input: 15 bits packed in a uint16 (MSB-aligned: bits 14..0).
// Returns: (11-bit data, corrected bool).
// Can detect 2-bit errors and correct 1-bit errors.
func Hamming1511Decode(codeword uint16) (uint16, bool) {
	// Parity check matrix for (15,11)
	// Syndrome = H * received^T
	s := hammingSyndrome15(codeword)
	if s == 0 {
		// No errors
		return (codeword >> 4) & 0x7FF, true
	}

	// Single-bit error correction
	pos := hammingSyndromeToPos15[s]
	if pos > 0 {
		codeword ^= 1 << uint(15-pos)
		return (codeword >> 4) & 0x7FF, true
	}

	// Uncorrectable (2+ errors)
	return (codeword >> 4) & 0x7FF, false
}

func hammingSyndrome15(codeword uint16) uint8 {
	// (15,11) Hamming: 4 parity bits
	// H matrix columns are the binary representations of 1-15
	var syndrome uint8
	for i := 0; i < 15; i++ {
		if codeword&(1<<uint(14-i)) != 0 {
			syndrome ^= uint8(i + 1)
		}
	}
	return syndrome
}

// hammingSyndromeToPos15 maps syndrome to bit position (1-indexed) for (15,11).
// Position 0 means uncorrectable.
var hammingSyndromeToPos15 = func() [16]int {
	var table [16]int
	for i := 1; i <= 15; i++ {
		table[i] = i
	}
	return table
}()

// Hamming106Decode decodes a (10,6,3) Hamming codeword.
// Input: 10 bits packed in a uint16 (bits 9..0).
// Returns: (6-bit data, corrected bool).
func Hamming106Decode(codeword uint16) (uint8, bool) {
	s := hammingSyndrome10(codeword)
	if s == 0 {
		return uint8((codeword >> 4) & 0x3F), true
	}

	pos := hammingSyndromeToPos10[s]
	if pos > 0 {
		codeword ^= 1 << uint(10-pos)
		return uint8((codeword >> 4) & 0x3F), true
	}

	return uint8((codeword >> 4) & 0x3F), false
}

func hammingSyndrome10(codeword uint16) uint8 {
	var syndrome uint8
	for i := 0; i < 10; i++ {
		if codeword&(1<<uint(9-i)) != 0 {
			syndrome ^= uint8(i + 1)
		}
	}
	return syndrome & 0x0F
}

// hammingSyndromeToPos10 maps syndrome to bit position for (10,6).
var hammingSyndromeToPos10 = func() [16]int {
	var table [16]int
	for i := 1; i <= 10; i++ {
		if i <= 15 {
			table[i&0xF] = i
		}
	}
	return table
}()
