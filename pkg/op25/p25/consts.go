package p25

// P25 Frame Sync Word (48 bits = 24 dibits)
// Hex: 0x5575F5FF77FF
const P25SyncWord uint64 = 0x5575F5FF77FF

// P25SyncDibits is the sync word as a sequence of dibit values (0-3).
// Derived from 0x5575F5FF77FF split into 2-bit pairs.
var P25SyncDibits = [SyncLengthDibits]byte{
	1, 1, 1, 1, 1, 3, 1, 1, 3, 3, 1, 1,
	3, 3, 3, 3, 1, 3, 1, 3, 3, 3, 3, 3,
}

const (
	// Sync word length
	SyncLengthDibits = 24
	SyncLengthBits   = 48

	// NID length (Golay-coded)
	NIDLengthDibits = 32
	NIDLengthBits   = 64

	// Maximum bit errors allowed in sync correlation
	SyncMaxErrors = 4

	// P25 symbol rate
	P25SymbolRate = 4800
)

// DUID identifies the P25 data unit type (4 bits from NID).
type DUID uint8

const (
	DUIDHeaderDataUnit   DUID = 0x0 // HDU - voice header
	DUIDTerminator       DUID = 0x3 // TDU - voice terminator
	DUIDLogicalLinkData1 DUID = 0x5 // LDU1 - voice frame 1
	DUIDTSDU             DUID = 0x7 // TSDU - trunking signaling data unit
	DUIDLogicalLinkData2 DUID = 0xA // LDU2 - voice frame 2
	DUIDTerminatorWithLC DUID = 0xF // TDULC - terminator with link control
)

// Frame sizes in dibits (total frame including sync + NID).
const (
	TSDUFrameDibits  = 720
	HDUFrameDibits   = 648
	LDUFrameDibits   = 1728
	TDUFrameDibits   = 72
	TDULCFrameDibits = 432
)

// FrameSizeForDUID returns the total frame size in dibits for a given DUID,
// or 0 if the DUID is unknown.
func FrameSizeForDUID(duid DUID) int {
	switch duid {
	case DUIDTSDU:
		return TSDUFrameDibits
	case DUIDHeaderDataUnit:
		return HDUFrameDibits
	case DUIDLogicalLinkData1, DUIDLogicalLinkData2:
		return LDUFrameDibits
	case DUIDTerminator:
		return TDUFrameDibits
	case DUIDTerminatorWithLC:
		return TDULCFrameDibits
	default:
		return 0
	}
}

// TSBK opcodes (8-bit opcode field)
const (
	TSBKOpcodeGrpVChGrant     uint8 = 0x00 // Group Voice Channel Grant
	TSBKOpcodeGrpVChGrantUpdt uint8 = 0x02 // Group Voice Channel Grant Update
	TSBKOpcodeUUVChGrant      uint8 = 0x04 // Unit-to-Unit Voice Channel Grant
	TSBKOpcodeUUAnsReq        uint8 = 0x05 // Unit-to-Unit Answer Request
	TSBKOpcodeUUVChGrantUpdt  uint8 = 0x06 // Unit-to-Unit Voice Channel Grant Update
	TSBKOpcodeTelIntVChGrant  uint8 = 0x08 // Telephone Interconnect Voice Channel Grant
	TSBKOpcodeGrpAffiliation  uint8 = 0x28 // Group Affiliation Response
	TSBKOpcodeUnitRegistration uint8 = 0x2C // Unit Registration Response
	TSBKOpcodeIdenUp          uint8 = 0x3D // Identifier Update
	TSBKOpcodeIdenUpVUHP      uint8 = 0x34 // Identifier Update for VHF/UHF bands
	TSBKOpcodeSysServiceBcast uint8 = 0x38 // System Service Broadcast
	TSBKOpcodeNetStatBcast    uint8 = 0x3B // Network Status Broadcast
	TSBKOpcodeRFSSStatBcast   uint8 = 0x3C // RFSS Status Broadcast
	TSBKOpcodeAdjStStatBcast  uint8 = 0x3E // Adjacent Status Broadcast
)

// TSBK constants
const (
	TSBKLength      = 12  // bytes (96 bits)
	TSBKDataBits    = 96  // total TSBK bits (opcode + data + CRC)
	TSBKsPerTSDU    = 3   // maximum TSBKs per TSDU frame
	TSBKCodedDibits = 196 // trellis-coded dibits per TSBK
)

// Status symbol positions in TSDU payload (dibits after NID).
// Status symbols occur every 35 data dibits in the payload area.
const StatusSymbolInterval = 35

// LDU IMBE voice codeword positions.
// Each LDU frame contains 9 IMBE voice codewords at specific dibit offsets
// (relative to start of frame, after sync+NID).
// Each IMBE codeword is 144 bits = 72 dibits.
const IMBECodewordBits = 144
const IMBECodewordsPerLDU = 9
