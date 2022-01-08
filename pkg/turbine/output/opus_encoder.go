package output

import (
	"context"
	"time"

	"github.com/hraban/opus"
	"github.com/norasector/turbine-common/types"
	"golang.org/x/sync/errgroup"
)

const usPerFrame int = 40e3

var validUsRates []int = []int{2.5e3, 5e3, 10e3, 20e3} //, 40e3}

type OpusEncoder struct {
	sampleRate    int
	tgid          int
	systemID      int
	outBuf        [16384]byte
	encBuf        [4096]byte
	inBuf         [4096]float32
	inBufPos      int
	outBufPos     int
	outBufCount   int
	encoder       *opus.Encoder
	segmentNumber int
	lastTG        types.TalkGroup

	outputChan chan *types.TaggedAudioFrameOpus

	receiveChan chan *types.TaggedAudioSampleFloat32
}

func NewOpusEncoder(sampleRate, systemID, tgid int, outputChan chan *types.TaggedAudioFrameOpus) (*OpusEncoder, error) {
	enc, err := opus.NewEncoder(sampleRate, 1, opus.AppVoIP)
	if err != nil {
		return nil, err
	}

	if err := enc.SetPacketLossPerc(20); err != nil {
		return nil, err
	}
	enc.SetBitrateToAuto()
	return &OpusEncoder{
		sampleRate:  sampleRate,
		systemID:    systemID,
		tgid:        tgid,
		receiveChan: make(chan *types.TaggedAudioSampleFloat32, 1),
		outputChan:  outputChan,
		encoder:     enc,
	}, nil
}

func (o *OpusEncoder) Start(ctx context.Context) error {
	eg, ctx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Microsecond * time.Duration(usPerFrame) * 3 / 2):
				if err := o.maybeFlush(ctx, true); err != nil {
					return err
				}
			case seg := <-o.receiveChan:
				copy(o.inBuf[o.inBufPos:o.inBufPos+len(seg.Audio.Data)], seg.Audio.Data)
				o.inBufPos += len(seg.Audio.Data)
				o.lastTG = *seg.TalkGroup
				if err := o.maybeFlush(ctx, false); err != nil {
					return err
				}
			}
		}
	})

	return eg.Wait()
}

func (o *OpusEncoder) maybeFlush(ctx context.Context, force bool) error {

	samplesPerFrame := o.sampleRate * usPerFrame / 1e6

	if o.inBufPos > samplesPerFrame || (force && o.inBufPos > 0) {
		if force {
			// create smaller segment size
			set := false
			for j := len(validUsRates) - 1; j >= 0; j-- {
				thisFrameCount := validUsRates[j] * o.sampleRate / 1e6
				if thisFrameCount < o.inBufPos {
					samplesPerFrame = thisFrameCount
					set = true
					break
				}
			}
			// too short, just throw it away
			if !set {
				o.inBufPos = 0
				o.outBufPos = 0
				o.outBufCount = 0
				return nil
			}
		}

		inputSample := o.inBuf[:samplesPerFrame]
		bytesEncoded, err := o.encoder.EncodeFloat32(inputSample, o.encBuf[0:4096])
		if err != nil {
			return err
		}

		// Move leftover samples to beginning of input buffer and reset position
		o.inBufPos = o.inBufPos - samplesPerFrame
		copy(o.inBuf[0:o.inBufPos], o.inBuf[samplesPerFrame:samplesPerFrame+o.inBufPos])

		// Copy encoded to end of output buffer
		copy(o.outBuf[o.outBufPos:o.outBufPos+bytesEncoded], o.encBuf[0:bytesEncoded])
		o.outBufCount++
		o.outBufPos += bytesEncoded

		ret := make([]byte, o.outBufPos)
		copy(ret, o.outBuf[0:o.outBufPos])

		o.outBufCount = 0
		o.outBufPos = 0

		select {
		case <-ctx.Done():
			return ctx.Err()
		case o.outputChan <- &types.TaggedAudioFrameOpus{
			Audio: &types.SegmentBinaryBytes{
				SegmentNumber: o.segmentNumber,
				Data:          ret,
			},
			TalkGroup:                &o.lastTG,
			SampleLengthMicroseconds: samplesPerFrame * 1e6 / o.sampleRate,
			Timestamp:                time.Now().UTC()}:
			o.segmentNumber++
		}
	}
	return nil
}

func (o *OpusEncoder) ReceiveChannel() chan<- *types.TaggedAudioSampleFloat32 {
	return o.receiveChan
}
