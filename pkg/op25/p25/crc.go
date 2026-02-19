package p25

// CRC-CCITT (CRC-16) used by P25 for TSBK validation.
// Polynomial: x^16 + x^12 + x^5 + 1 (0x1021)
// Initial value: 0x0000 (P25 uses 0-init per TIA-102.BAAA)

const crcPoly uint16 = 0x1021

// CRC16 computes the CRC-CCITT over the given data bits.
// Each byte in data should contain a single bit (0 or 1).
func CRC16(data []byte) uint16 {
	var crc uint16 = 0x0000
	for _, bit := range data {
		feedback := ((crc >> 15) ^ uint16(bit&1)) & 1
		crc <<= 1
		if feedback == 1 {
			crc ^= crcPoly
		}
	}
	return crc & 0xFFFF
}

// CRC16Bytes computes CRC-CCITT over packed bytes (MSB first).
func CRC16Bytes(data []byte, nbits int) uint16 {
	var crc uint16 = 0x0000
	bitCount := 0
	for _, b := range data {
		for i := 7; i >= 0 && bitCount < nbits; i-- {
			bit := (b >> uint(i)) & 1
			feedback := ((crc >> 15) ^ uint16(bit)) & 1
			crc <<= 1
			if feedback == 1 {
				crc ^= crcPoly
			}
			bitCount++
		}
	}
	return crc & 0xFFFF
}

// CheckCRC16 validates that data (including CRC) has CRC == 0.
// data is a bit array (1 bit per byte), length should be data_bits + 16 CRC bits.
func CheckCRC16(data []byte) bool {
	return CRC16(data) == 0
}
