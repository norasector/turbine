package p25

import (
	"github.com/norasector/turbine-common/types"
	p25 "github.com/norasector/turbine/pkg/op25/p25"
	"github.com/norasector/turbine/pkg/op25/vocoder"
	"github.com/racerxdl/segdsp/dsp"
	"github.com/rs/zerolog"
)

// VoiceAssembler handles P25 voice frame assembly and IMBE decoding.
// It receives dibits from the DSP pipeline, performs frame sync,
// extracts IMBE codewords from LDU1/LDU2 frames, decodes them via
// the vocoder, resamples, and outputs audio samples.
type VoiceAssembler struct {
	logger     zerolog.Logger
	voc        vocoder.Vocoder
	outputChan chan<- *types.TaggedAudioSampleFloat32
	systemID   int
	frequency  int
	resampler  *dsp.FloatResampler

	outputSampleRate int

	// Frame sync
	sync *p25.FrameSync

	// Frame accumulation
	frameBuf    []byte
	frameSize   int
	collecting  bool
	currentNAC  uint16
	currentDUID p25.DUID
}

// NewVoiceAssembler creates a P25 voice frame assembler.
// voc is the IMBE vocoder, outputChan receives decoded audio,
// outputSampleRate is the target sample rate (e.g., 24000 Hz).
func NewVoiceAssembler(
	voc vocoder.Vocoder,
	outputChan chan<- *types.TaggedAudioSampleFloat32,
	systemID int,
	outputSampleRate int,
	logger zerolog.Logger,
) *VoiceAssembler {
	// IMBE produces 8000 Hz, resample to output rate
	resampleRatio := float32(outputSampleRate) / 8000.0

	return &VoiceAssembler{
		logger:           logger,
		voc:              voc,
		outputChan:       outputChan,
		systemID:         systemID,
		outputSampleRate: outputSampleRate,
		resampler:        dsp.MakeFloatResampler(63, resampleRatio),
		sync:             p25.NewFrameSync(),
		frameBuf:         make([]byte, 0, p25.LDUFrameDibits),
	}
}

// SetFrequency updates the current voice channel frequency.
func (va *VoiceAssembler) SetFrequency(freq int) {
	va.frequency = freq
}

// Receive processes dibits (values 0-3) from the DSP pipeline.
func (va *VoiceAssembler) Receive(dibits []byte) {
	for _, d := range dibits {
		va.receiveDibit(d & 0x03)
	}
}

func (va *VoiceAssembler) receiveDibit(dibit byte) {
	if va.collecting {
		va.frameBuf = append(va.frameBuf, dibit)
		if len(va.frameBuf) >= va.frameSize {
			va.processFrame()
			va.collecting = false
			va.frameBuf = va.frameBuf[:0]
		}
		return
	}

	result := va.sync.Feed(dibit)
	if !result.SyncFound {
		return
	}

	// Start collecting NID
	va.frameBuf = va.frameBuf[:0]
	va.collecting = true
	va.frameSize = p25.NIDLengthDibits
}

func (va *VoiceAssembler) processFrame() {
	if len(va.frameBuf) < p25.NIDLengthDibits {
		return
	}

	// Phase 1: decode NID
	if len(va.frameBuf) == p25.NIDLengthDibits {
		nid := p25.DecodeNID(va.frameBuf[:p25.NIDLengthDibits])
		if !nid.Valid {
			va.collecting = false
			va.frameBuf = va.frameBuf[:0]
			return
		}

		va.currentNAC = nid.NAC
		va.currentDUID = nid.DUID

		totalFrameDibits := p25.FrameSizeForDUID(nid.DUID)
		if totalFrameDibits == 0 {
			va.collecting = false
			va.frameBuf = va.frameBuf[:0]
			return
		}

		payloadSize := totalFrameDibits - p25.SyncLengthDibits
		if payloadSize <= p25.NIDLengthDibits {
			va.processVoiceFrame()
			return
		}

		va.frameSize = payloadSize
		va.collecting = true
		return
	}

	// Phase 2: full frame received
	va.processVoiceFrame()
}

func (va *VoiceAssembler) processVoiceFrame() {
	switch va.currentDUID {
	case p25.DUIDLogicalLinkData1, p25.DUIDLogicalLinkData2:
		va.processLDU()
	case p25.DUIDHeaderDataUnit:
		va.logger.Debug().Str("system", "p25").Msg("voice HDU received")
	case p25.DUIDTerminator, p25.DUIDTerminatorWithLC:
		va.logger.Debug().Str("system", "p25").Msg("voice terminator received")
	}
}

// LDU frame IMBE codeword extraction offsets.
// Each LDU frame (after sync+NID) contains 9 IMBE voice codewords
// interspersed with link control, low-speed data, and status symbols.
//
// LDU1 structure (1728 dibits total, 1704 after sync):
//   NID (32) + VC1(72) + ... + VC9(72) + LC fields + status symbols
//
// The exact dibit offsets of each voice codeword within the payload
// (after NID, before status symbol removal):
var lduIMBEOffsets = [p25.IMBECodewordsPerLDU]int{
	// Approximate offsets in dibits from start of payload (after NID)
	// These account for interleaved link control and status symbols
	0, 120, 240, 360, 480, 600, 720, 840, 960,
}

// IMBE codeword size in dibits (144 bits = 72 dibits)
const imbeCodewordDibits = p25.IMBECodewordBits / 2

func (va *VoiceAssembler) processLDU() {
	if va.voc == nil {
		return
	}

	payload := va.frameBuf[p25.NIDLengthDibits:]

	// Remove status symbols
	cleaned := removeStatusSymbols(payload)

	// Extract and decode 9 IMBE codewords
	var allSamples []float32

	for i := 0; i < p25.IMBECodewordsPerLDU; i++ {
		offset := lduIMBEOffsets[i]
		if offset+imbeCodewordDibits > len(cleaned) {
			break
		}

		codewordDibits := cleaned[offset : offset+imbeCodewordDibits]

		// Convert dibits to bits (1 bit per byte) for the vocoder
		imbeBits := make([]byte, p25.IMBECodewordBits)
		for j := 0; j < imbeCodewordDibits; j++ {
			imbeBits[j*2] = (codewordDibits[j] >> 1) & 1
			imbeBits[j*2+1] = codewordDibits[j] & 1
		}

		// Decode IMBE → PCM (160 samples at 8kHz)
		pcm, err := va.voc.Decode(imbeBits[:88]) // mbelib expects 88 bits
		if err != nil {
			va.logger.Trace().Err(err).Int("codeword", i).Msg("IMBE decode error")
			continue
		}

		allSamples = append(allSamples, pcm...)
	}

	if len(allSamples) == 0 {
		return
	}

	// Resample 8kHz → output sample rate
	resampled := make([]float32, len(allSamples)*va.outputSampleRate/8000+1)
	n := va.resampler.WorkBuffer(allSamples, resampled)
	resampled = resampled[:n]

	// Output audio
	select {
	case va.outputChan <- &types.TaggedAudioSampleFloat32{
		TalkGroup: &types.TalkGroup{
			SystemID: va.systemID,
		},
		Audio: &types.SegmentFloat32{
			Frequency: va.frequency,
			Data:      resampled,
		},
	}:
	default:
		// Channel full, drop
	}
}

// Close releases vocoder resources.
func (va *VoiceAssembler) Close() {
	if va.voc != nil {
		va.voc.Close()
	}
}
