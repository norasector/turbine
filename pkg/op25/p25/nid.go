package p25

// NID (Network ID) decoder for P25.
// The NID is a 64-bit field (32 dibits) that follows the frame sync word.
// It contains:
//   - NAC: 12-bit Network Access Code (like a system ID)
//   - DUID: 4-bit Data Unit ID (identifies frame type)
//   - Parity: BCH(63,16,23) + 1 overall parity bit
//
// Structure: [BCH codeword (63 bits) | overall parity (1 bit)]
// BCH data: NAC(12) << 4 | DUID(4) = 16 info bits at positions 62-47.

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

	// Decode using BCH(63,16,23)
	nac, duid, _, valid := BCHDecodeNID(nidBits)
	if valid {
		return NID{NAC: nac, DUID: duid, Valid: true}
	}

	// Fallback: direct extraction with DUID sanity check
	return decodeNIDDirect(nidBits)
}

// decodeNIDDirect attempts NID extraction without full BCH decode.
// Falls back to raw bit extraction which may have errors.
func decodeNIDDirect(nidBits uint64) NID {
	// The 64-bit NID: [BCH(63) | parity(1)]
	// BCH info bits at positions 62-47 of the 63-bit codeword
	// = positions 63-48 of the 64-bit NID (shifted left by 1 for parity)
	cw := nidBits >> 1
	infoBits := uint16(cw >> 47)
	nac := (infoBits >> 4) & 0xFFF
	duid := DUID(infoBits & 0x0F)

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
