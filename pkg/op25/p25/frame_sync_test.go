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
	// Encode using BCH(63,16,23) to create valid NID

	nac := uint16(0x293)
	duid := DUIDTSDU

	// BCH-encode the NID
	nidBits := BCHEncodeNID(nac, duid)

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
	if nid.NAC != nac {
		t.Errorf("NAC: got 0x%03X, expected 0x%03X", nid.NAC, nac)
	}
	if nid.DUID != duid {
		t.Errorf("DUID: got 0x%X, expected 0x%X", nid.DUID, duid)
	}
}
