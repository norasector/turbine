package p25

import "testing"

func TestBCHGenPoly(t *testing.T) {
	// Verify degree is 47 (63 - 16 = 47 parity bits)
	deg := 0
	for i := 63; i >= 0; i-- {
		if bchGenPoly&(1<<uint(i)) != 0 {
			deg = i
			break
		}
	}
	if deg != 47 {
		t.Errorf("generator poly degree = %d, expected 47", deg)
	}

	// Verify constant term is 1
	if bchGenPoly&1 == 0 {
		t.Error("generator poly constant term should be 1")
	}

	// Verify matches known value from p25.rs
	expected := uint64(0xCD930BDD3B2B)
	if bchGenPoly != expected {
		t.Errorf("bchGenPoly = 0x%012X, expected 0x%012X", bchGenPoly, expected)
	}
}

func TestBCHEncode(t *testing.T) {
	// Test vector from p25.rs: encode(0xFF00) = 0xFF009310C2306868
	cw := BCHEncode63(0xFF00)
	nid := (cw << 1) | (popcount64(cw) & 1)
	expected := uint64(0xFF009310C2306868)
	if nid != expected {
		t.Errorf("BCHEncodeNID(0xFF00): got 0x%016X, expected 0x%016X", nid, expected)
		t.Logf("  BCH codeword: 0x%016X", cw)
		t.Logf("  Expected cw:  0x%016X", expected>>1)
	}
}

func TestBCHEncodeDecode(t *testing.T) {
	// Test several info values
	testVals := []uint16{0x0000, 0xFFFF, 0xFF00, 0x2937, 0x1234, 0xABCD}
	for _, data := range testVals {
		cw := BCHEncode63(data)
		decoded, nerrs, valid := BCHDecode63(cw)
		if !valid {
			t.Errorf("BCHDecode63 failed for data=0x%04X", data)
			continue
		}
		if nerrs != 0 {
			t.Errorf("BCHDecode63 reported %d errors for clean codeword (data=0x%04X)", nerrs, data)
		}
		if decoded != data {
			t.Errorf("BCHDecode63 decoded 0x%04X, expected 0x%04X", decoded, data)
		}
	}
}

func TestBCHSingleBitError(t *testing.T) {
	data := uint16(0x2937)
	cw := BCHEncode63(data)

	// Flip each of the 63 bit positions
	for bit := 0; bit < 63; bit++ {
		corrupted := cw ^ (1 << uint(bit))
		decoded, _, valid := BCHDecode63(corrupted)
		if !valid {
			t.Errorf("BCHDecode63 failed for single-bit error at pos %d", bit)
			continue
		}
		if decoded != data {
			t.Errorf("BCHDecode63 decoded 0x%04X, expected 0x%04X (bit %d flipped)", decoded, data, bit)
		}
	}
}

func TestBCHMultiBitErrors(t *testing.T) {
	data := uint16(0xABCD)
	cw := BCHEncode63(data)

	// Test 3-bit error patterns
	errorSets := [][3]int{
		{0, 12, 23},
		{5, 30, 55},
		{47, 50, 62},
		{1, 31, 61},
	}
	for _, errs := range errorSets {
		corrupted := cw ^ (1 << uint(errs[0])) ^ (1 << uint(errs[1])) ^ (1 << uint(errs[2]))
		decoded, nerr, valid := BCHDecode63(corrupted)
		if !valid {
			t.Errorf("BCHDecode63 failed for 3-bit error at %v", errs)
			continue
		}
		if decoded != data {
			t.Errorf("BCHDecode63 decoded 0x%04X, expected 0x%04X (bits %v flipped)", decoded, data, errs)
		}
		if nerr != 3 {
			t.Errorf("BCHDecode63 reported %d errors, expected 3", nerr)
		}
	}
}

func TestBCHEncodeDecodeNID(t *testing.T) {
	nac := uint16(0x293)
	duid := DUIDTSDU

	nid := BCHEncodeNID(nac, duid)
	decNAC, decDUID, nerrs, valid := BCHDecodeNID(nid)
	if !valid {
		t.Fatal("BCHDecodeNID failed for clean NID")
	}
	if nerrs != 0 {
		t.Errorf("BCHDecodeNID reported %d errors for clean NID", nerrs)
	}
	if decNAC != nac {
		t.Errorf("NAC: got 0x%03X, expected 0x%03X", decNAC, nac)
	}
	if decDUID != duid {
		t.Errorf("DUID: got 0x%X, expected 0x%X", decDUID, duid)
	}
}
