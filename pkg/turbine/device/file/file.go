package file

import (
	"context"
	"os"
	"time"

	"github.com/norasector/turbine-common/types"
)

type FileDevice struct {
	readFile    *os.File
	readSize    int
	timeBetween time.Duration
	outputChan  chan *types.SegmentComplex64
	sampleRate  int
	centerFreq  int
}

func NewFileDevice(file string, readSize int, sampleRate int, centerFreq int, timeBetween time.Duration) (*FileDevice, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}

	return &FileDevice{
		readFile:    f,
		readSize:    readSize,
		timeBetween: timeBetween,
		sampleRate:  sampleRate,
		centerFreq:  centerFreq,
	}, nil

}

func (f *FileDevice) Start(ctx context.Context, centerFreq int, sampleRate int, complexSamples chan *types.SegmentComplex64) error {
	tick := time.NewTicker(f.timeBetween)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
			buf := make([]byte, f.readSize)
			n, err := f.readFile.Read(buf)
			if err != nil {
				return err
			}

			seg := types.SegmentCS8Raw{
				SampleRate: f.sampleRate,
				Data:       make([]byte, len(buf[:n])),
				Frequency:  f.centerFreq,
			}
			copy(seg.Data, buf)

			complexSegment := seg.ToComplex64()

			select {
			case <-ctx.Done():
				return ctx.Err()
			case complexSamples <- complexSegment:
			}

		}
	}

}

func (f *FileDevice) Stop() error {
	return f.readFile.Close()
}
func (f *FileDevice) MaxSampleRate() int {
	return 20e6
}
