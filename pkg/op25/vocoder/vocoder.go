package vocoder

// Vocoder is the interface for P25 voice decoding.
// Different implementations can wrap mbelib, codec2, or software IMBE decoders.
type Vocoder interface {
	// Decode takes an IMBE frame (88 bits, 1 bit per byte) and returns
	// 160 float32 PCM samples at 8kHz, normalized to [-1.0, 1.0].
	Decode(imbeFrame []byte) ([]float32, error)

	// Close releases resources held by the vocoder.
	Close()
}
