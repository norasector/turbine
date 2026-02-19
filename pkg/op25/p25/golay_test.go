package p25

import "testing"

func TestGolayEncodeDecode(t *testing.T) {
	// Test all possible 12-bit data values (subset)
	testVals := []uint32{0x000, 0x001, 0x555, 0xAAA, 0xFFF, 0x123, 0x7E4}
	for _, data := range testVals {
		encoded := GolayEncode24(data)
		decoded, errs, valid := GolayDecode24(encoded)
		if !valid {
			t.Errorf("GolayDecode24 failed for data=0x%03X, encoded=0x%06X", data, encoded)
			continue
		}
		if errs != 0 {
			t.Errorf("GolayDecode24 reported %d errors for clean codeword (data=0x%03X)", errs, data)
		}
		if decoded != data {
			t.Errorf("GolayDecode24 decoded 0x%03X, expected 0x%03X", decoded, data)
		}
	}
}

func TestGolaySingleBitError(t *testing.T) {
	data := uint32(0x5A3)
	encoded := GolayEncode24(data)

	// Flip each bit position and verify correction
	for bit := 0; bit < 24; bit++ {
		corrupted := encoded ^ (1 << uint(bit))
		decoded, _, valid := GolayDecode24(corrupted)
		if !valid {
			t.Errorf("GolayDecode24 failed to correct single-bit error at pos %d", bit)
			continue
		}
		if decoded != data {
			t.Errorf("GolayDecode24 decoded 0x%03X, expected 0x%03X (bit %d flipped)", decoded, data, bit)
		}
	}
}

func TestGolayTwoBitErrors(t *testing.T) {
	data := uint32(0xABC)
	encoded := GolayEncode24(data)

	// Test a few two-bit error patterns
	errorPairs := [][2]int{{0, 1}, {5, 10}, {11, 23}, {3, 20}}
	for _, pair := range errorPairs {
		corrupted := encoded ^ (1 << uint(pair[0])) ^ (1 << uint(pair[1]))
		decoded, _, valid := GolayDecode24(corrupted)
		if !valid {
			t.Errorf("GolayDecode24 failed to correct 2-bit error at pos %d,%d", pair[0], pair[1])
			continue
		}
		if decoded != data {
			t.Errorf("GolayDecode24 decoded 0x%03X, expected 0x%03X (bits %d,%d flipped)", decoded, data, pair[0], pair[1])
		}
	}
}

func TestGolayThreeBitErrors(t *testing.T) {
	data := uint32(0x321)
	encoded := GolayEncode24(data)

	// Three-bit error pattern
	corrupted := encoded ^ (1 << 0) ^ (1 << 12) ^ (1 << 23)
	decoded, _, valid := GolayDecode24(corrupted)
	if !valid {
		t.Errorf("GolayDecode24 failed to correct 3-bit error")
		return
	}
	if decoded != data {
		t.Errorf("GolayDecode24 decoded 0x%03X, expected 0x%03X", decoded, data)
	}
}
