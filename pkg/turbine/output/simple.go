package output

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"time"

	"github.com/norasector/turbine-common/types"
	"golang.org/x/sync/errgroup"
)

const sampleBufferLength int = 8

type SimpleAudioOutput struct {
	dest            io.Writer
	recvChan        chan *types.TaggedAudioSampleFloat32
	outChan         chan *types.TaggedAudioSampleFloat32
	sampleRate      int
	sampleWaitTime  time.Duration
	talkGroupFilter map[int]struct{}
}

func NewSimpleAudioOutput(dest io.Writer, sampleRate int, talkGroups []int) *SimpleAudioOutput {
	ret := &SimpleAudioOutput{
		dest:            dest,
		sampleRate:      sampleRate,
		recvChan:        make(chan *types.TaggedAudioSampleFloat32, sampleBufferLength),
		outChan:         make(chan *types.TaggedAudioSampleFloat32, sampleBufferLength),
		sampleWaitTime:  time.Second,
		talkGroupFilter: make(map[int]struct{}),
	}

	for _, tg := range talkGroups {
		ret.talkGroupFilter[tg] = struct{}{}
	}

	return ret
}

func (s *SimpleAudioOutput) Receive() chan<- *types.TaggedAudioSampleFloat32 {
	return s.recvChan
}

func (s *SimpleAudioOutput) Start(ctx context.Context) error {
	eg, ctx := errgroup.WithContext(ctx)

	sampleLen := 785
	singleSampleWaitTime := time.Duration(1000 / float64(s.sampleRate) * float64(time.Millisecond))

	if _, err := s.dest.Write(make([]byte, s.sampleRate*4)); err != nil {
		return err
	}

	// Concurrently filter incoming samples to only get what we are looking for.
	for i := 0; i < 4; i++ {
		eg.Go(func() error {
			for {
				select {
				case <-ctx.Done():
					return ctx.Err()

				case ts := <-s.recvChan:
					if _, ok := s.talkGroupFilter[ts.TalkGroup.ID]; !ok {
						continue
					}

					select {
					case <-ctx.Done():
						return ctx.Err()
					case s.outChan <- ts:
					}

				}
			}
		})
	}

	eg.Go(func() error {
		var b *bytes.Buffer
		bufNum := 0

		for {
			select {
			case <-ctx.Done():
				return ctx.Err()

			case <-time.After(singleSampleWaitTime * time.Duration(sampleLen*(sampleBufferLength-bufNum))):
				if bufNum > 0 {
					if _, err := b.WriteTo(s.dest); err != nil {
						return err
					}
					b.Reset()
					bufNum = 0
				}

			case outBuf := <-s.outChan:
				sampleLen = len(outBuf.Audio.Data)
				if b == nil {
					b = bytes.NewBuffer(make([]byte, 0, sampleLen*4*sampleBufferLength+1))
				}

				if err := binary.Write(b, binary.LittleEndian, outBuf.Audio.Data); err != nil {
					return err
				}

				bufNum++
				if bufNum == sampleBufferLength {
					if _, err := b.WriteTo(s.dest); err != nil {
						return err
					}
					b.Reset()
					bufNum = 0
				}

			}

		}

	})

	return eg.Wait()
}
