package p25

import (
	"context"
	"time"

	"github.com/norasector/turbine/pkg/op25"
	p25 "github.com/norasector/turbine/pkg/op25/p25"
	"github.com/rs/zerolog"
)

// P25Assembler implements the frame.Assembler interface for P25 control channels.
// It receives dibits from the DSP pipeline, performs frame sync detection,
// NID decoding, and TSBK extraction from TSDU frames.
type P25Assembler struct {
	systemID   int
	outputChan chan op25.OSWPacket
	logger     zerolog.Logger
	ctx        context.Context

	// Frame sync state
	sync *p25.FrameSync

	// Frame accumulation
	frameBuf   []byte // accumulates dibits for current frame
	frameSize  int    // expected frame size in dibits
	collecting bool   // true when accumulating frame dibits
	currentNAC uint16
	currentDUID p25.DUID

	// Stats
	syncCount int
	tsbkCount int
}

// NewP25Assembler creates a new P25 control channel frame assembler.
func NewP25Assembler(ctx context.Context, systemID int, ch chan op25.OSWPacket, logger zerolog.Logger) *P25Assembler {
	return &P25Assembler{
		systemID:   systemID,
		outputChan: ch,
		logger:     logger,
		ctx:        ctx,
		sync:       p25.NewFrameSync(),
		frameBuf:   make([]byte, 0, p25.LDUFrameDibits), // allocate for largest possible frame
	}
}

// Receive implements frame.Assembler.
// Accepts dibits (values 0-3, one per byte) from the dibit slicer.
func (a *P25Assembler) Receive(buf []byte) {
	for _, dibit := range buf {
		a.receiveDibit(dibit & 0x03)
	}
}

func (a *P25Assembler) receiveDibit(dibit byte) {
	if a.collecting {
		a.frameBuf = append(a.frameBuf, dibit)
		if len(a.frameBuf) >= a.frameSize {
			a.processFrame()
			a.collecting = false
			a.frameBuf = a.frameBuf[:0]
		}
		return
	}

	// Feed to sync detector
	result := a.sync.Feed(dibit)
	if !result.SyncFound {
		return
	}

	a.syncCount++

	// Sync found — next 32 dibits are the NID
	// We need to accumulate NID + payload, so start collecting from here
	// The sync detector consumed the sync word; now we collect the rest of the frame.
	// However, our sync detector works on a per-dibit basis, so at this point
	// we haven't yet collected the NID. We need to buffer NID dibits first.
	a.frameBuf = a.frameBuf[:0]
	a.collecting = true

	// We'll collect NID + payload.
	// Max frame size (without sync) is LDU at 1728 - 24 = 1704 dibits.
	// For now, collect NID first, then determine payload size.
	a.frameSize = p25.NIDLengthDibits // start by collecting just the NID
}

func (a *P25Assembler) processFrame() {
	if len(a.frameBuf) < p25.NIDLengthDibits {
		return
	}

	// If we just collected the NID, decode it and start collecting payload
	if len(a.frameBuf) == p25.NIDLengthDibits {
		nid := p25.DecodeNID(a.frameBuf[:p25.NIDLengthDibits])
		if !nid.Valid {
			a.collecting = false
			a.frameBuf = a.frameBuf[:0]
			return
		}

		a.currentNAC = nid.NAC
		a.currentDUID = nid.DUID

		// Determine how much more to collect based on DUID
		totalFrameDibits := p25.FrameSizeForDUID(nid.DUID)
		if totalFrameDibits == 0 {
			a.logger.Debug().
				Uint8("duid", uint8(nid.DUID)).
				Msg("unknown P25 DUID")
			a.collecting = false
			a.frameBuf = a.frameBuf[:0]
			return
		}

		// Frame size = total - sync(24) = remaining after sync
		// We already have NID(32), so remaining = total - sync(24) - NID is already in buf
		// Actually: total includes sync(24) + NID(32) + payload
		// We need to collect: total - sync(24) dibits (NID + payload)
		payloadSize := totalFrameDibits - p25.SyncLengthDibits
		if payloadSize <= p25.NIDLengthDibits {
			// Frame too small (e.g., TDU = 72 total - 24 sync = 48, but NID is 32)
			// Process what we have
			a.processFramePayload()
			return
		}

		// Continue collecting
		a.frameSize = payloadSize
		a.collecting = true
		return
	}

	// Full frame collected (NID + payload)
	a.processFramePayload()
}

func (a *P25Assembler) processFramePayload() {
	switch a.currentDUID {
	case p25.DUIDTSDU:
		a.processTSDU()
	default:
		// For control channel, we only care about TSDU frames
		a.logger.Trace().
			Uint8("duid", uint8(a.currentDUID)).
			Uint16("nac", a.currentNAC).
			Msg("non-TSDU frame on control channel")
	}
}

// processTSDU extracts up to 3 TSBKs from a TSDU frame.
func (a *P25Assembler) processTSDU() {
	// Frame buffer contains: NID(32 dibits) + payload
	// Payload starts after NID
	if len(a.frameBuf) < p25.NIDLengthDibits {
		return
	}

	payload := a.frameBuf[p25.NIDLengthDibits:]

	// Remove status symbols from payload.
	// Status symbols appear at regular intervals in the payload.
	cleaned := removeStatusSymbols(payload)

	// Extract up to 3 TSBKs from the cleaned payload.
	// Each TSBK is 196 coded dibits (after status removal).
	for i := 0; i < p25.TSBKsPerTSDU; i++ {
		offset := i * p25.TSBKCodedDibits
		if offset+p25.TSBKCodedDibits > len(cleaned) {
			break
		}

		tsbkDibits := cleaned[offset : offset+p25.TSBKCodedDibits]
		tsbk, ok := DecodeTSBK(tsbkDibits)
		if !ok {
			continue
		}

		a.tsbkCount++
		a.logger.Debug().
			Uint8("opcode", tsbk.Opcode).
			Uint16("nac", a.currentNAC).
			Bool("last_block", tsbk.LastBlock).
			Msg("P25 TSBK decoded")

		select {
		case <-a.ctx.Done():
			return
		case a.outputChan <- op25.OSWPacket{
			SystemID:   a.systemID,
			SystemType: op25.SystemTypeP25,
			Packet: P25ControlPacket{
				NAC:  a.currentNAC,
				DUID: a.currentDUID,
				TSBK: tsbk,
			},
			Timestamp: time.Now().UTC(),
		}:
		}

		if tsbk.LastBlock {
			break
		}
	}
}

// removeStatusSymbols strips status dibits that are interspersed in the payload.
// Status symbols appear every StatusSymbolInterval data dibits.
func removeStatusSymbols(payload []byte) []byte {
	result := make([]byte, 0, len(payload))
	dataCount := 0
	for _, d := range payload {
		dataCount++
		if dataCount%(p25.StatusSymbolInterval+1) == 0 {
			// This is a status symbol, skip it
			continue
		}
		result = append(result, d)
	}
	return result
}
