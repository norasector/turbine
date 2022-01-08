package turbine

import (
	"context"
	"fmt"
	"math"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go"
	"github.com/norasector/turbine-common/types"
	"github.com/norasector/turbine/pkg/dsp/agc/rmsagc"
	"github.com/norasector/turbine/pkg/dsp/demodulators/quad"
	"github.com/norasector/turbine/pkg/dsp/filters/fir"
	"github.com/norasector/turbine/pkg/dsp/mixer"
	"github.com/norasector/turbine/pkg/dsp/processor"
	"github.com/norasector/turbine/pkg/op25"
	"github.com/norasector/turbine/pkg/op25/frame"
	"github.com/norasector/turbine/pkg/op25/frame/smartnet"
	"github.com/norasector/turbine/pkg/op25/modem/fsk4"
	"github.com/norasector/turbine/pkg/op25/slicer"
	"github.com/norasector/turbine/pkg/util"
	"github.com/racerxdl/segdsp/dsp"
)

const (
	ifRate = 18000 // target rate for processing symbols, 18000/3600=5
)

func NewControlFrequency(
	t *Turbine,
	sys *internalSystem,
	freq int,
) *ControlFrequency {

	f := &ControlFrequency{
		SystemID:   sys.ID,
		SymbolRate: sys.SymbolRate,
		Frequency:  freq,
		SystemType: sys.SystemType,
	}

	f.init(t, sys)

	return f
}

func (freq *ControlFrequency) init(t *Turbine, sys *internalSystem) {
	if freq.initialized {
		return
	}

	switch freq.SystemType {
	case op25.SystemTypeSmartnet:
		freq.initSmartnet(t, sys)
	default:
		panic(fmt.Errorf("unknown system type %s", freq.SystemType))
	}

	freq.initialized = true
}

type ControlFrequency struct {
	SystemID int

	SystemType op25.SystemType

	Frequency  int
	SymbolRate int

	initialized bool

	sampleNum int

	proc *processor.Processor

	assembler frame.Assembler
}

func (t *Turbine) processControlChannel(ctx context.Context, buf *types.SegmentComplex64, freq *ControlFrequency) error {
	start := time.Now()
	metrics := map[string]interface{}{
		"sample_length": len(buf.Data),
		"sample_bytes":  len(buf.Data) * 8,
	}

	defer func() {

		metrics["duration"] = time.Since(start).Microseconds()

		go t.writeAPI.WritePoint(influxdb2.NewPoint("control.processed",
			map[string]string{
				"frequency":    op25.MHzToString(freq.Frequency),
				"sample_type":  "complex64",
				"channel_type": "control",
			},
			metrics, start))
	}()

	sliced, err := freq.proc.ProcessComplexToBinary(buf, metrics)
	if err != nil {
		return err
	}

	metrics["assembler_duration"] = util.TimeOperationMicroseconds(func() {
		freq.assembler.Receive(sliced.Data)
	})

	freq.sampleNum++

	return nil
}

func (freq *ControlFrequency) initSmartnet(t *Turbine, sys *internalSystem) {

	freq.proc = processor.NewProcessor(fmt.Sprintf("%d-control-%d", sys.ID, freq.Frequency), "Radio Input", t.vizServer)

	var dec1, dec2 int

	switch t.opts.SampleRate {
	case 10e6:
		dec1 = 25
		dec2 = 16
	case 8e6:
		dec1 = 20
		dec2 = 16
	default:
		dec1 = 10
		if t.opts.SampleRate > 1e6 {
			dec1 *= t.opts.SampleRate / 1e6
		}
		dec2 = 4
	}

	if1 := float64(t.opts.SampleRate) / float64(dec1)
	if2 := float64(if1) / float64(dec2)

	// Offset from center
	shiftFreq := freq.Frequency - t.opts.CenterFreq

	bfoFreq := float64(shiftFreq) / if1
	bfoFreq -= math.Floor(bfoFreq)
	if bfoFreq < -0.5 {
		bfoFreq += 1.0
	}
	if bfoFreq > 0.5 {
		bfoFreq -= 1.0
	}

	t.logger.Info().
		Int("system_id", freq.SystemID).
		Str("frequency", op25.MHzToString(freq.Frequency)).
		Str("channel_type", "control").
		Int("decimation_1", dec1).
		Int("decimation_2", dec2).
		Int("intermediate_freq_1", int(if1)).
		Int("intermediate_freq_2", int(if2)).
		Int("intermediate_rate", ifRate).
		Str("shift_freq", op25.MHzToString(shiftFreq)).
		Str("bfo_freq", op25.MHzToString(int(if1*bfoFreq))).
		Msg("initializing channel")

	bpfCoeffs := fir.MakeComplexBandPass(1.0,
		float64(t.opts.SampleRate),
		float64(shiftFreq)-if1/2.0,
		float64(shiftFreq)+if1/2.0,
		if1/2,
		fir.Hamming,
	)

	// 8M / 80 = 100k
	// freq.bandpassDecimator =

	freq.proc.AddBlock(processor.NewDSPWorkerCC(
		"bandpass_decimator",
		"Bandpass Decimator",
		t.opts.SampleRate,
		int(if1),
		dsp.MakeDecimationCTFirFilter(dec1, bpfCoeffs),
	))

	// Low pass -> get to 100k
	// Band pass -> get to 25k

	// Beat frequency oscillator -- shifts down to actual channel frequency
	freq.proc.AddBlock(processor.NewDSPWorkerCC(
		"bfo_mixer",
		"BFO Mixer",
		int(if1),
		int(if1),
		mixer.NewWaveformMixer(int(if1), int(if1*bfoFreq)),
	))

	// Mixer -- band pass
	fa := float64(6250)
	fb := if2 / 2

	lpfCoeffs := fir.MakeLowPass(1.0, if1, (fb+fa)/2, fb-fa, fir.Hamming)
	freq.proc.AddBlock(processor.NewDSPWorkerCC(
		"lowpass_decimator",
		"Lowpass Decimator",
		int(if1),
		int(if2),
		dsp.MakeDecimationFirFilter(dec2, lpfCoeffs),
	))

	freq.proc.AddBlock(processor.NewDSPWorkerCC(
		"resampler",
		"Rational Resampler",
		int(if2),
		int(ifRate),
		dsp.MakeRationalResampler(int(ifRate)/1000, int(if2)/1000),
	))

	fa = 6250
	fb = fa + 625

	cutoffLpfCoeffs := fir.MakeLowPass(1.0,
		ifRate,
		(fb+fa)/2,
		fb-fa,
		fir.Hann)

	freq.proc.AddBlock(processor.NewDSPWorkerCC(
		"cutoff",
		"Cutoff Filter",
		int(ifRate),
		int(ifRate),
		dsp.MakeFirFilter(cutoffLpfCoeffs),
	))

	freq.proc.AddBlock(processor.NewDSPWorkerCF(
		"quad_demod",
		"FM Demodulation",
		int(ifRate),
		int(ifRate),
		quad.MakeQuadDemod(
			ifRate/(2*math.Pi*float32(freq.SymbolRate)),
		)))

	freq.proc.AddBlock(processor.NewDSPWorkerFF(
		"baseband_amp",
		"Baseband Amp (RMS AGC)",
		int(ifRate),
		int(ifRate),
		rmsagc.NewRMSAGC(0.01, 0.61)))

	sps := ifRate / freq.SymbolRate
	ntaps := (7 * sps) | 1
	symbolFilterTaps := dsp.MakeRRC(1.0, float64(ifRate), float64(freq.SymbolRate), 0.35, ntaps)

	freq.proc.AddBlock(processor.NewDSPWorkerFF(
		"symbol_filter",
		"Symbol Filter (RRC)",
		int(ifRate),
		int(ifRate),
		dsp.MakeFloatFirFilter(symbolFilterTaps)))

	freq.proc.AddBlock(processor.NewDSPWorkerFF(
		"fsk_demodulator",
		"FSK Demodulator (BFSK)",
		int(ifRate),
		freq.SymbolRate,
		fsk4.NewFSK4Demodulator(ifRate, freq.SymbolRate, true),
		processor.WithVizLength(26),
	))

	freq.proc.AddBlock(processor.NewDSPWorkerFB(
		"binary_slicer",
		"Binary Slicer",
		freq.SymbolRate,
		freq.SymbolRate,
		slicer.NewBinarySlicer(true)))

	freq.assembler = smartnet.NewSmartnetAssembler(t.ctx, freq.SystemID, sys.dataPacketChan, t.logger)
}
