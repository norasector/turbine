package p25

// Half-rate Trellis Coded Modulation decoder for P25.
// Used for decoding TSBK data blocks in TSDU frames.
//
// The P25 trellis code:
// - Rate 1/2 at the dibit level (1 input dibit → 2 output dibits)
// - 4 states (2-bit state register)
// - Each constellation pair (2 dibits) maps to (next_state, decoded_dibit)
// - Decoded via Viterbi algorithm
//
// Input: 196 coded dibits (98 constellation pairs)
// Output: 96 bits (12 bytes) of TSBK data
//
// Reference: TIA-102.BAAA §7.2

const (
	trellisStates        = 4
	trellisInputDibits   = 49  // 48 data dibits + 1 tail dibit
	trellisCodedDibits   = 196 // total coded dibits per TSBK
	trellisConstellPairs = 98  // 196 / 2 constellation pairs
)

// Constellation pair mapping table.
// Index: [state][input_dibit] → output constellation pair (as 2 dibits packed: high_dibit<<2 | low_dibit)
// The output is a 4-bit value representing a pair of dibits.
var trellisEncodeTable = [trellisStates][4]uint8{
	// State 0: input dibits 0,1,2,3 → constellation pairs
	{0x02, 0x0C, 0x01, 0x0F},
	// State 1
	{0x0E, 0x00, 0x0D, 0x03},
	// State 2
	{0x09, 0x07, 0x0A, 0x04},
	// State 3
	{0x05, 0x0B, 0x06, 0x08},
}

// trellisNextState maps [current_state][input_dibit] → next state
var trellisNextState = [trellisStates][4]uint8{
	{0, 1, 2, 3},
	{0, 1, 2, 3},
	{0, 1, 2, 3},
	{0, 1, 2, 3},
}

// constellationDistance computes the Hamming distance between two constellation pairs
// (each represented as a 4-bit value).
func constellationDistance(a, b uint8) int {
	x := a ^ b
	d := 0
	for x != 0 {
		d += int(x & 1)
		x >>= 1
	}
	return d
}

const maxMetric = 0x7FFFFFFF

// TrellisDecode decodes 196 coded dibits into 96 bits (12 bytes) of data.
// Returns (decoded bytes, success).
func TrellisDecode(codedDibits []byte) ([TSBKLength]byte, bool) {
	var result [TSBKLength]byte

	if len(codedDibits) < trellisCodedDibits {
		return result, false
	}

	// Convert dibits to constellation pairs
	pairs := make([]uint8, trellisConstellPairs)
	for i := 0; i < trellisConstellPairs; i++ {
		d0 := codedDibits[i*2] & 0x03
		d1 := codedDibits[i*2+1] & 0x03
		pairs[i] = (d0 << 2) | d1
	}

	// Viterbi algorithm
	// Path metrics: [state] → accumulated distance
	metrics := [trellisStates]int{0, maxMetric, maxMetric, maxMetric}
	newMetrics := [trellisStates]int{}

	// Traceback: [step][state] → previous state
	traceback := make([][trellisStates]uint8, trellisConstellPairs)
	// Decoded dibit at each step for traceback
	decodedAt := make([][trellisStates]uint8, trellisConstellPairs)

	for step := 0; step < trellisConstellPairs; step++ {
		receivedPair := pairs[step]

		for s := 0; s < trellisStates; s++ {
			newMetrics[s] = maxMetric
		}

		for prevState := 0; prevState < trellisStates; prevState++ {
			if metrics[prevState] >= maxMetric {
				continue
			}
			for inputDibit := 0; inputDibit < 4; inputDibit++ {
				nextState := int(trellisNextState[prevState][inputDibit])
				expectedPair := trellisEncodeTable[prevState][inputDibit]
				dist := constellationDistance(expectedPair, receivedPair)
				candidate := metrics[prevState] + dist

				if candidate < newMetrics[nextState] {
					newMetrics[nextState] = candidate
					traceback[step][nextState] = uint8(prevState)
					decodedAt[step][nextState] = uint8(inputDibit)
				}
			}
		}

		metrics = newMetrics
	}

	// Find best ending state (should be state 0 for proper termination)
	bestState := 0
	bestMetric := metrics[0]
	for s := 1; s < trellisStates; s++ {
		if metrics[s] < bestMetric {
			bestMetric = metrics[s]
			bestState = s
		}
	}

	// Traceback to recover decoded dibits
	decoded := make([]uint8, trellisConstellPairs)
	state := bestState
	for step := trellisConstellPairs - 1; step >= 0; step-- {
		decoded[step] = decodedAt[step][state]
		state = int(traceback[step][state])
	}

	// Convert decoded dibits to bytes
	// First 48 dibits are data (96 bits), last dibit is tail
	bits := make([]byte, 0, TSBKDataBits)
	for i := 0; i < 48 && i < len(decoded); i++ {
		bits = append(bits, (decoded[i]>>1)&1)
		bits = append(bits, decoded[i]&1)
	}

	// Pack bits into bytes
	for i := 0; i < TSBKLength && i*8+7 < len(bits); i++ {
		var b byte
		for j := 0; j < 8; j++ {
			b = (b << 1) | (bits[i*8+j] & 1)
		}
		result[i] = b
	}

	return result, bestMetric < 40 // allow some errors but reject garbage
}

// TrellisDecodeFromBits is a convenience wrapper that accepts a bit array
// (1 bit per byte) instead of dibits.
func TrellisDecodeFromBits(bits []byte) ([TSBKLength]byte, bool) {
	if len(bits) < trellisCodedDibits*2 {
		return [TSBKLength]byte{}, false
	}

	dibits := make([]byte, trellisCodedDibits)
	for i := 0; i < trellisCodedDibits; i++ {
		dibits[i] = (bits[i*2]<<1 | bits[i*2+1]) & 0x03
	}
	return TrellisDecode(dibits)
}
