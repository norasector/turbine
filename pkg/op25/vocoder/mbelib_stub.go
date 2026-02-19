//go:build !cgo

package vocoder

import "fmt"

// MBELibDecoder is a stub for when CGo is not available.
type MBELibDecoder struct{}

func NewMBELibDecoder() (*MBELibDecoder, error) {
	return nil, fmt.Errorf("mbelib vocoder requires CGo (build with CGO_ENABLED=1)")
}

func (d *MBELibDecoder) Decode(imbeBits []byte) ([]float32, error) {
	return nil, fmt.Errorf("mbelib not available")
}

func (d *MBELibDecoder) Close() {}
