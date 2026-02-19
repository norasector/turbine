package p25

// P25 frame synchronization.
// Correlates incoming dibits against the 48-bit P25 sync pattern.
// Uses a sliding window correlation with configurable error threshold.

// FrameSync tracks dibit input and detects the P25 48-bit sync word.
type FrameSync struct {
	// Circular buffer of recent dibits for correlation
	buf      [SyncLengthDibits]byte
	bufIdx   int
	count    int // total dibits received
	inSync   bool
	framePos int // position within current frame (dibits since last sync)
}

// NewFrameSync creates a new P25 frame synchronizer.
func NewFrameSync() *FrameSync {
	return &FrameSync{}
}

// Reset clears the frame sync state.
func (fs *FrameSync) Reset() {
	fs.bufIdx = 0
	fs.count = 0
	fs.inSync = false
	fs.framePos = 0
	for i := range fs.buf {
		fs.buf[i] = 0
	}
}

// SyncResult is the outcome of feeding a dibit to the synchronizer.
type SyncResult struct {
	SyncFound bool // true when sync word detected
	Errors    int  // number of bit errors in sync correlation (-1 if no sync)
}

// Feed processes one dibit (value 0-3) and checks for sync.
// Returns true if sync word is detected at the current position.
func (fs *FrameSync) Feed(dibit byte) SyncResult {
	fs.buf[fs.bufIdx] = dibit & 0x03
	fs.bufIdx = (fs.bufIdx + 1) % SyncLengthDibits
	fs.count++
	fs.framePos++

	// Need at least a full sync word buffer
	if fs.count < SyncLengthDibits {
		return SyncResult{Errors: -1}
	}

	errors := fs.correlate()
	if errors <= SyncMaxErrors {
		fs.framePos = 0
		fs.inSync = true
		return SyncResult{SyncFound: true, Errors: errors}
	}

	return SyncResult{Errors: errors}
}

// InSync returns true if currently synchronized.
func (fs *FrameSync) InSync() bool {
	return fs.inSync
}

// FramePos returns the current dibit position within the frame
// (0 = just after sync word was detected).
func (fs *FrameSync) FramePos() int {
	return fs.framePos
}

// SetLostSync marks synchronization as lost.
func (fs *FrameSync) SetLostSync() {
	fs.inSync = false
}

// correlate computes the number of bit errors between the buffer contents
// and the P25 sync word. Each dibit is compared as 2 bits.
func (fs *FrameSync) correlate() int {
	errors := 0
	for i := 0; i < SyncLengthDibits; i++ {
		// Read from circular buffer in order
		idx := (fs.bufIdx + i) % SyncLengthDibits
		received := fs.buf[idx]
		expected := P25SyncDibits[i]

		// Compare as 2-bit values (XOR and count set bits)
		diff := received ^ expected
		if diff&0x02 != 0 {
			errors++
		}
		if diff&0x01 != 0 {
			errors++
		}
	}
	return errors
}

// CorrelateBuffer checks a buffer of dibits for the sync word at position 0.
// Returns the number of bit errors.
func CorrelateBuffer(dibits []byte) int {
	if len(dibits) < SyncLengthDibits {
		return SyncLengthBits // max errors
	}

	errors := 0
	for i := 0; i < SyncLengthDibits; i++ {
		diff := (dibits[i] & 0x03) ^ P25SyncDibits[i]
		if diff&0x02 != 0 {
			errors++
		}
		if diff&0x01 != 0 {
			errors++
		}
	}
	return errors
}
