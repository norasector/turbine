//go:build cgo

package vocoder

/*
#cgo LDFLAGS: -lmbe
#cgo CFLAGS: -I/usr/local/include

#include <stdlib.h>

// mbelib IMBE decoder functions
// See: https://github.com/szechyjs/mbelib
typedef struct {
	int cur_mp;
	int prev_mp;
	float audio_out_temp_buf_p[160];
	float audio_out_temp_buf[160];
	float audio_out_float_buf[160];
	int errs;
	int errs2;
	int err_str[7];
	char ambe_d[49];
	int b[9];
	int L;
	int K;
	int Vl[57];
	float w0;
	float phi[57];
	float psi1;
	float log2Ml[57];
	float gamma;
	float Ml[57];
	float mus;
	float prev_mus;
} mbe_parms;

extern void mbe_initMbeParms(mbe_parms *cur_mp, mbe_parms *prev_mp, mbe_parms *prev_mp_enhanced);
extern int mbe_processImbe4400Dataf(float *aout_buf, int *errs, int *errs2, char *err_str,
                                     char imbe_d[88], mbe_parms *cur_mp, mbe_parms *prev_mp,
                                     mbe_parms *prev_mp_enhanced, int uvquality);
*/
import "C"
import (
	"fmt"
	"unsafe"
)

// MBELibDecoder wraps the mbelib C library for IMBE 4400 voice decoding.
type MBELibDecoder struct {
	curMP           C.mbe_parms
	prevMP          C.mbe_parms
	prevMPEnhanced  C.mbe_parms
}

// NewMBELibDecoder creates and initializes a new mbelib IMBE decoder.
func NewMBELibDecoder() (*MBELibDecoder, error) {
	d := &MBELibDecoder{}
	C.mbe_initMbeParms(&d.curMP, &d.prevMP, &d.prevMPEnhanced)
	return d, nil
}

// Decode decodes an IMBE frame (88 bits as char array) into PCM audio.
// Returns 160 float32 samples at 8kHz.
func (d *MBELibDecoder) Decode(imbeBits []byte) ([]float32, error) {
	if len(imbeBits) < 88 {
		return nil, fmt.Errorf("IMBE frame too short: got %d bits, need 88", len(imbeBits))
	}

	var aoutBuf [160]C.float
	var errs, errs2 C.int
	var errStr [7]C.char

	// Copy IMBE bits to C array
	var imbeD [88]C.char
	for i := 0; i < 88; i++ {
		imbeD[i] = C.char(imbeBits[i])
	}

	C.mbe_processImbe4400Dataf(
		(*C.float)(unsafe.Pointer(&aoutBuf[0])),
		&errs,
		&errs2,
		&errStr[0],
		&imbeD[0],
		&d.curMP,
		&d.prevMP,
		&d.prevMPEnhanced,
		3, // uvquality
	)

	// Convert C float array to Go float32 slice
	// mbelib outputs int16-range values; normalize to [-1.0, 1.0]
	output := make([]float32, 160)
	for i := 0; i < 160; i++ {
		output[i] = float32(aoutBuf[i]) / 32768.0
	}

	return output, nil
}

// Close releases any resources held by the decoder.
func (d *MBELibDecoder) Close() {
	// mbelib doesn't have explicit cleanup
}
