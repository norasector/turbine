package frame

// Assembler takes decoded bytes demodulated over the air and assembles them into packets.
type Assembler interface {
	// Receive expects a buffer of 1s and 0s that correspond to the bits in a packet.
	// Each byte should only contain 1 bit.  There is no bit packing.
	Receive([]byte)
}
