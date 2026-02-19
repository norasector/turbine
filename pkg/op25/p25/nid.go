package p25

// NID (Network ID) decoder for P25.
// The NID is a 64-bit field (32 dibits) that follows the frame sync word.
// It contains:
//   - NAC: 12-bit Network Access Code (like a system ID)
//   - DUID: 4-bit Data Unit ID (identifies frame type)
//   - Parity: 48 bits of Golay coding
//
// The 64 bits are encoded as two Golay(24,12) codewords:
//   - First 24 bits: Golay codeword containing NAC[11:0] (12 data bits)
//   - Next 24 bits:  Golay codeword containing DUID[3:0] + reserved/parity (12 data bits)
//   - Remaining 16 bits: additional parity/BCH bits
//
// Simplified approach: extract NAC and DUID from the 64-bit NID using Golay decoding.

// NID represents a decoded P25 Network ID.
type NID struct {
	NAC   uint16 // 12-bit Network Access Code
	DUID  DUID   // 4-bit Data Unit ID
	Valid bool   // Whether decoding succeeded
}

// DecodeNID decodes a 64-bit NID from 32 dibits.
// Each dibit is a byte value 0-3.
func DecodeNID(dibits []byte) NID {
	if len(dibits) < NIDLengthDibits {
		return NID{}
	}

	// Convert dibits to a 64-bit value
	var nidBits uint64
	for i := 0; i < NIDLengthDibits; i++ {
		nidBits = (nidBits << 2) | uint64(dibits[i]&0x03)
	}

	// Extract the two 24-bit Golay codewords
	// The NID is structured as: [NAC(12) | DUID(4) | parity(48)]
	// Encoded as Golay(24,12) blocks

	// First Golay block: bits 63..40 (24 bits)
	cw1 := uint32((nidBits >> 40) & 0xFFFFFF)
	data1, _, ok1 := GolayDecode24(cw1)

	// Second Golay block: bits 39..16 (24 bits)
	cw2 := uint32((nidBits >> 16) & 0xFFFFFF)
	data2, _, ok2 := GolayDecode24(cw2)

	if !ok1 || !ok2 {
		// Try alternative NID structure: direct extraction with error tolerance
		return decodeNIDDirect(nidBits)
	}

	// data1 contains NAC (12 bits)
	nac := uint16(data1 & 0xFFF)
	// data2 contains DUID in the upper 4 bits (of 12 decoded bits)
	duid := DUID((data2 >> 8) & 0x0F)

	return NID{
		NAC:   nac,
		DUID:  duid,
		Valid: true,
	}
}

// decodeNIDDirect attempts NID extraction without full Golay decode.
// Falls back to raw bit extraction which may have errors.
func decodeNIDDirect(nidBits uint64) NID {
	// NAC is in bits [63:52] (top 12 bits)
	nac := uint16((nidBits >> 52) & 0xFFF)
	// DUID is in bits [51:48]
	duid := DUID((nidBits >> 48) & 0x0F)

	// Basic sanity check on DUID
	switch duid {
	case DUIDHeaderDataUnit, DUIDTerminator, DUIDLogicalLinkData1,
		DUIDTSDU, DUIDLogicalLinkData2, DUIDTerminatorWithLC:
		return NID{NAC: nac, DUID: duid, Valid: true}
	}

	return NID{}
}

// DecodeNIDFromBits decodes a NID from 64 individual bits (1 bit per byte).
func DecodeNIDFromBits(bits []byte) NID {
	if len(bits) < NIDLengthBits {
		return NID{}
	}

	// Pack bits into dibits
	dibits := make([]byte, NIDLengthDibits)
	for i := 0; i < NIDLengthDibits; i++ {
		dibits[i] = ((bits[i*2] & 1) << 1) | (bits[i*2+1] & 1)
	}
	return DecodeNID(dibits)
}
