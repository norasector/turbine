package hackrf

import (
	"context"
	"os"

	"github.com/norasector/turbine-common/types"
	"github.com/samuel/go-hackrf/hackrf"
)

const maxSampleRate = 20e6

func (r *HackRFDevice) MaxSampleRate() int {
	return maxSampleRate
}

type HackRFDevice struct {
	device *hackrf.Device

	centerFreq int
	sampleRate int

	outputChan chan *types.SegmentComplex64
	ctx        context.Context

	recordLocation string
	outputFile     *os.File
}

func NewRecordingHackRFDevice(recordLocation string) (*HackRFDevice, error) {

	device, err := hackrf.Open()
	if err != nil {
		return nil, err
	}

	outFile, err := os.Create(recordLocation)
	if err != nil {
		return nil, err
	}

	return &HackRFDevice{
		device:         device,
		outputFile:     outFile,
		recordLocation: recordLocation,
	}, nil
}

func NewHackRFDevice() (*HackRFDevice, error) {

	device, err := hackrf.Open()
	if err != nil {
		return nil, err
	}

	return &HackRFDevice{
		device: device,
	}, nil
}

func (h *HackRFDevice) callback(buf []byte) error {
	if h.outputFile != nil {
		if _, err := h.outputFile.Write(buf); err != nil {
			return err
		}

		return nil
	}
	seg := types.SegmentCS8Raw{
		SampleRate: h.sampleRate,
		Data:       make([]byte, len(buf)),
		Frequency:  h.centerFreq,
	}
	copy(seg.Data, buf)

	complexSegment := seg.ToComplex64()
	select {
	case <-h.ctx.Done():
		return h.ctx.Err()
	case h.outputChan <- complexSegment:
	}

	return nil
}

func (h *HackRFDevice) Start(ctx context.Context, centerFreq int, sampleRate int, complexSamples chan *types.SegmentComplex64) error {
	h.ctx = ctx
	h.outputChan = complexSamples
	h.centerFreq = centerFreq
	h.sampleRate = sampleRate
	if err := h.device.SetFreq(uint64(h.centerFreq)); err != nil {
		return err
	}
	if err := h.device.SetSampleRateManual(h.sampleRate*2, 2); err != nil {
		return err
	}

	if err := h.device.SetLNAGain(39); err != nil {
		return err
	}
	if err := h.device.SetBasebandFilterBandwidth(h.sampleRate); err != nil {
		return err
	}
	if err := h.device.SetAmpEnable(true); err != nil {
		return err
	}
	return h.device.StartRX(h.callback)
}

func (h *HackRFDevice) Stop() error {
	if h.outputFile != nil {
		defer h.outputFile.Close()
	}
	return h.device.StopRX()
}
