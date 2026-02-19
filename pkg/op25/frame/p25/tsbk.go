package p25

import (
	p25 "github.com/norasector/turbine/pkg/op25/p25"
)

// TSBK represents a decoded Trunking Signaling Block.
type TSBK struct {
	Opcode    uint8
	MFR       uint8    // Manufacturer ID (0x00 = standard)
	Data      [8]byte  // Operand data (variable format per opcode)
	CRCValid  bool
	LastBlock bool // Last Block flag (indicates final TSBK in multi-block message)
	Raw       [p25.TSBKLength]byte
}

// DecodeTSBK takes 196 coded dibits, trellis-decodes them, validates CRC,
// and parses the TSBK fields.
func DecodeTSBK(codedDibits []byte) (TSBK, bool) {
	decoded, ok := p25.TrellisDecode(codedDibits)
	if !ok {
		return TSBK{}, false
	}

	// Validate CRC-CCITT over 80 data bits (bytes 0-9), CRC in bytes 10-11
	// The TSBK structure (96 bits = 12 bytes):
	//   Byte 0:    [LB(1) | Protected(1) | Opcode(6)]
	//   Byte 1:    MFR ID
	//   Bytes 2-9: Operand data (64 bits)
	//   Bytes 10-11: CRC-CCITT (16 bits)

	// Verify CRC over first 10 bytes (80 bits)
	crc := p25.CRC16Bytes(decoded[:10], 80)
	expectedCRC := uint16(decoded[10])<<8 | uint16(decoded[11])

	tsbk := TSBK{
		LastBlock: (decoded[0] >> 7) & 1 == 1,
		Opcode:    decoded[0] & 0x3F,
		MFR:       decoded[1],
		CRCValid:  crc == expectedCRC,
		Raw:       decoded,
	}
	copy(tsbk.Data[:], decoded[2:10])

	return tsbk, tsbk.CRCValid
}

// IdenEntry stores frequency band identifier information from IDEN_UP TSBKs.
type IdenEntry struct {
	Identifier uint8
	BaseFreq   int64 // Hz
	Spacing    int   // Hz
	Offset     int   // Hz (transmit offset)
	Bandwidth  int   // Hz
}

// IdenTable maps identifier values to frequency band entries.
type IdenTable map[uint8]IdenEntry

// ChannelToFrequency converts a P25 channel number to a frequency (Hz)
// using the identifier table.
func (t IdenTable) ChannelToFrequency(channel uint16) int {
	iden := uint8((channel >> 12) & 0x0F)
	chNum := channel & 0x0FFF

	entry, ok := t[iden]
	if !ok {
		return 0
	}

	return int(entry.BaseFreq) + entry.Spacing*int(chNum)
}

// ParseIdenUp extracts frequency band definition from an IDEN_UP TSBK.
// Opcode 0x3D: Identifier Update
// Data layout (64 bits = Data[0:8]):
//   Bits 63-60: Identifier (4)
//   Bits 59-51: BW (9)
//   Bits 50-42: TX Offset (9)
//   Bits 41-32: Channel Spacing (10)
//   Bits 31-0:  Base Frequency (32)
func ParseIdenUp(tsbk TSBK) IdenEntry {
	data := tsbk.Data[:]

	iden := (data[0] >> 4) & 0x0F
	bw := (uint16(data[0]&0x0F) << 5) | uint16(data[1]>>3)
	offset := uint16(data[1]&0x07)<<6 | uint16(data[2]>>2)
	spacing := uint32(data[2]&0x03)<<8 | uint32(data[3])
	baseFreq := uint32(data[4])<<24 | uint32(data[5])<<16 | uint32(data[6])<<8 | uint32(data[7])

	return IdenEntry{
		Identifier: iden,
		BaseFreq:   int64(baseFreq) * 5, // base freq is in units of 5 Hz
		Spacing:    int(spacing) * 125,   // spacing is in units of 125 Hz
		Offset:     int(offset) * 250000, // offset in 250 kHz steps
		Bandwidth:  int(bw) * 125,        // bandwidth in units of 125 Hz
	}
}

// ParseIdenUpVUHP extracts frequency band definition from an IDEN_UP_VU TSBK.
// Opcode 0x34: Identifier Update for VHF/UHF bands.
// Same bit layout as IDEN_UP.
func ParseIdenUpVUHP(tsbk TSBK) IdenEntry {
	data := tsbk.Data[:]

	iden := (data[0] >> 4) & 0x0F
	bw := (uint16(data[0]&0x0F) << 5) | uint16(data[1]>>3)
	offset := uint16(data[1]&0x07)<<6 | uint16(data[2]>>2)
	spacing := uint32(data[2]&0x03)<<8 | uint32(data[3])
	baseFreq := uint32(data[4])<<24 | uint32(data[5])<<16 | uint32(data[6])<<8 | uint32(data[7])

	return IdenEntry{
		Identifier: iden,
		BaseFreq:   int64(baseFreq) * 5,
		Spacing:    int(spacing) * 125,
		Offset:     int(offset) * 250000,
		Bandwidth:  int(bw) * 125,
	}
}

// P25ControlPacket wraps a TSBK for transport via the OSWPacket system.
type P25ControlPacket struct {
	NAC  uint16
	DUID p25.DUID
	TSBK TSBK
}

// GrpVChGrant represents a parsed Group Voice Channel Grant.
type GrpVChGrant struct {
	Channel uint16
	TGID    uint16
	SrcID   uint32
}

// ParseGrpVChGrant extracts fields from a GRP_V_CH_GRANT TSBK (opcode 0x00).
// Data layout (Data[0:8] = TSBK bytes 2-9):
//   Data[0]:   Service Options
//   Data[1:2]: Channel (16 bits: 4-bit identifier + 12-bit channel number)
//   Data[3:4]: Group Address / TGID (16 bits)
//   Data[5:7]: Source Address / SrcID (24 bits)
func ParseGrpVChGrant(tsbk TSBK) GrpVChGrant {
	data := tsbk.Data[:]

	return GrpVChGrant{
		Channel: uint16(data[1])<<8 | uint16(data[2]),
		TGID:    uint16(data[3])<<8 | uint16(data[4]),
		SrcID:   uint32(data[5])<<16 | uint32(data[6])<<8 | uint32(data[7]),
	}
}

// ParseGrpVChGrantUpdt extracts fields from a GRP_V_CH_GRANT_UPDT TSBK (opcode 0x02).
// Contains two channel/TGID pairs.
type GrpVChGrantUpdt struct {
	Channel1 uint16
	TGID1    uint16
	Channel2 uint16
	TGID2    uint16
}

func ParseGrpVChGrantUpdt(tsbk TSBK) GrpVChGrantUpdt {
	data := tsbk.Data[:]

	return GrpVChGrantUpdt{
		Channel1: uint16(data[0])<<8 | uint16(data[1]),
		TGID1:    uint16(data[2])<<8 | uint16(data[3]),
		Channel2: uint16(data[4])<<8 | uint16(data[5]),
		TGID2:    uint16(data[6])<<8 | uint16(data[7]),
	}
}
