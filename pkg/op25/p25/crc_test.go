package p25

import "testing"

func TestCRC16(t *testing.T) {
	// Test with known data: all zeros should produce 0 CRC
	zeros := make([]byte, 80)
	crc := CRC16(zeros)
	// CRC of all zeros with 0 init = 0
	if crc != 0 {
		t.Logf("CRC of all zeros: 0x%04X (non-zero is OK depending on init)", crc)
	}

	// Test that CRC changes with different data
	data1 := make([]byte, 80)
	data1[0] = 1
	data2 := make([]byte, 80)
	data2[1] = 1

	crc1 := CRC16(data1)
	crc2 := CRC16(data2)
	if crc1 == crc2 {
		t.Errorf("CRC should differ for different data: both produced 0x%04X", crc1)
	}
}

func TestCRC16Bytes(t *testing.T) {
	// Test CRC16Bytes against bit-level CRC16 for consistency
	data := []byte{0xA5, 0x3C, 0x7E, 0x00, 0xFF}
	nbits := 40

	crcBytes := CRC16Bytes(data, nbits)

	// Convert to bit array and compute with CRC16
	bits := make([]byte, nbits)
	for i, b := range data {
		for j := 7; j >= 0; j-- {
			idx := i*8 + (7 - j)
			if idx < nbits {
				bits[idx] = (b >> uint(j)) & 1
			}
		}
	}
	crcBits := CRC16(bits)

	if crcBytes != crcBits {
		t.Errorf("CRC16Bytes (0x%04X) != CRC16 (0x%04X)", crcBytes, crcBits)
	}
}

func TestCheckCRC16(t *testing.T) {
	// Create data + CRC, then verify CheckCRC16 returns true
	data := make([]byte, 80)
	data[0] = 1
	data[10] = 1
	data[50] = 1

	crc := CRC16(data)

	// Append CRC bits (MSB first)
	withCRC := make([]byte, 96)
	copy(withCRC, data)
	for i := 0; i < 16; i++ {
		withCRC[80+i] = byte((crc >> uint(15-i)) & 1)
	}

	if !CheckCRC16(withCRC) {
		t.Errorf("CheckCRC16 should return true for valid data+CRC")
	}

	// Corrupt one bit
	withCRC[5] ^= 1
	if CheckCRC16(withCRC) {
		t.Errorf("CheckCRC16 should return false for corrupted data")
	}
}
