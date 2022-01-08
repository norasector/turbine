package rtlsdr

import (
	"context"
	"sync"

	gsdr "github.com/jpoirier/gortlsdr"
	"github.com/norasector/turbine-common/types"
)

const maxSampleRate = 2e6

type RTLSDRDevice struct {
	deviceIdx int
	device    *gsdr.Context

	centerFreq int
	sampleRate int

	outputChan chan *types.SegmentComplex64
	ctx        context.Context
	wg         sync.WaitGroup
}

func NewRTLSDRDevice(deviceIdx int) (*RTLSDRDevice, error) {
	return &RTLSDRDevice{deviceIdx: deviceIdx}, nil
}

func (r *RTLSDRDevice) MaxSampleRate() int {
	return maxSampleRate
}

func (r *RTLSDRDevice) callback(buf []byte) {
	r.wg.Add(1)
	defer r.wg.Done()
	seg := types.SegmentCS8Raw{
		SampleRate: r.sampleRate,
		Data:       buf,
		Frequency:  r.centerFreq,
	}

	complexSegment := seg.ToComplex64()
	select {
	case <-r.ctx.Done():
	case r.outputChan <- complexSegment:
	}
}

func (r *RTLSDRDevice) Stop() error {

	err := r.device.CancelAsync()

	r.wg.Wait()
	if err != nil {
		return err
	}

	return r.device.Close()
}

func (r *RTLSDRDevice) Start(ctx context.Context, centerFreq int, sampleRate int, complexSamples chan *types.SegmentComplex64) error {
	var err error
	r.device, err = gsdr.Open(r.deviceIdx)
	if err != nil {
		return err
	}
	r.ctx = ctx
	r.centerFreq = centerFreq
	r.sampleRate = sampleRate
	r.outputChan = complexSamples

	if err := r.device.SetCenterFreq(r.centerFreq); err != nil {
		return err
	}
	if err := r.device.SetSampleRate(r.sampleRate); err != nil {
		return err
	}
	if err := r.device.ResetBuffer(); err != nil {
		return err
	}

	r.wg.Add(1)
	defer r.wg.Done()
	return r.device.ReadAsync(r.callback, nil, 0, 0)
}
