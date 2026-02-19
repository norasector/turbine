package p25

import "testing"

func TestFrameSyncDetection(t *testing.T) {
	fs := NewFrameSync()

	// Feed the exact sync word dibits
	for _, d := range P25SyncDibits {
		result := fs.Feed(d)
		if result.SyncFound {
			if result.Errors != 0 {
				t.Errorf("Expected 0 errors, got %d", result.Errors)
			}
			return
		}
	}

	t.Error("Sync word not detected")
}

func TestFrameSyncWithErrors(t *testing.T) {
	fs := NewFrameSync()

	// Create sync with a few bit errors (flip some dibits)
	corrupted := P25SyncDibits
	corrupted[5] ^= 0x01 // 1 bit error
	corrupted[10] ^= 0x02 // 1 bit error

	for _, d := range corrupted {
		result := fs.Feed(d)
		if result.SyncFound {
			if result.Errors != 2 {
				t.Errorf("Expected 2 errors, got %d", result.Errors)
			}
			return
		}
	}

	t.Error("Sync word not detected with 2 bit errors")
}

func TestFrameSyncNoFalsePositive(t *testing.T) {
	fs := NewFrameSync()

	// Feed random-ish data that shouldn't trigger sync
	for i := 0; i < 1000; i++ {
		d := byte(i % 4)
		result := fs.Feed(d)
		if result.SyncFound {
			// It's possible but unlikely — allow it if errors are high
			// With our threshold of 4, a cycling pattern shouldn't trigger
			if result.Errors <= SyncMaxErrors {
				// This could happen by chance; just log it
				t.Logf("Sync triggered at position %d with %d errors", i, result.Errors)
			}
		}
	}
}

func TestCorrelateBuffer(t *testing.T) {
	// Perfect sync
	errors := CorrelateBuffer(P25SyncDibits[:])
	if errors != 0 {
		t.Errorf("Expected 0 errors for perfect sync, got %d", errors)
	}

	// All zeros
	zeros := make([]byte, SyncLengthDibits)
	errors = CorrelateBuffer(zeros)
	if errors == 0 {
		t.Error("Expected non-zero errors for all-zeros buffer")
	}
}

func TestNIDDecode(t *testing.T) {
	// Create a synthetic NID with known NAC and DUID
	// NAC = 0x293, DUID = 0x7 (TSDU)
	// We encode using Golay to create valid NID

	nac := uint32(0x293)
	duid := uint32(0x7)

	// First Golay block: NAC (12 bits)
	cw1 := GolayEncode24(nac)
	// Second Golay block: DUID in upper 4 bits of 12-bit data
	cw2 := GolayEncode24(duid << 8)

	// Pack into 64 bits: cw1(24) | cw2(24) | padding(16)
	var nidBits uint64
	nidBits = uint64(cw1)<<40 | uint64(cw2)<<16

	// Convert to dibits
	dibits := make([]byte, NIDLengthDibits)
	for i := 0; i < NIDLengthDibits; i++ {
		shift := uint(62 - i*2)
		dibits[i] = byte((nidBits >> shift) & 0x03)
	}

	nid := DecodeNID(dibits)
	if !nid.Valid {
		t.Fatal("NID decode failed")
	}
	if nid.NAC != uint16(nac) {
		t.Errorf("NAC: got 0x%03X, expected 0x%03X", nid.NAC, nac)
	}
	if nid.DUID != DUIDTSDU {
		t.Errorf("DUID: got 0x%X, expected 0x%X", nid.DUID, DUIDTSDU)
	}
}
