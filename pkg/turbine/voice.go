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
	"github.com/norasector/turbine/pkg/dsp/viz"
	"github.com/norasector/turbine/pkg/op25"
	framep25 "github.com/norasector/turbine/pkg/op25/frame/p25"
	"github.com/norasector/turbine/pkg/op25/modem/fsk4"
	"github.com/norasector/turbine/pkg/op25/p25"
	"github.com/norasector/turbine/pkg/op25/slicer"
	"github.com/norasector/turbine/pkg/op25/vocoder"
	"github.com/racerxdl/segdsp/dsp"
)

type VoiceFrequency struct {
	Frequency int
	Bandwidth int
	LastSeen  time.Time
	SystemID  int

	proc         *processor.Processor
	voiceDecoder *framep25.VoiceAssembler // nil for analog (SmartNet), non-nil for P25
}

func (freq *VoiceFrequency) initNBFM(t *Turbine, sys *internalSystem) {

	freq.proc = processor.NewProcessor(fmt.Sprintf("%d-voice-%d", sys.ID, freq.Frequency), "Radio Input", t.vizServer)

	var dec1, dec2 int

	switch t.opts.SampleRate {
	case 10e6:
		dec1 = 40
		dec2 = 20
	case 8e6:
		dec1 = 20
		dec2 = 32
	default:
		dec1 = 10 // 20 //20
		if t.opts.SampleRate > 1e6 {
			dec1 *= t.opts.SampleRate / 1e6
		}
		dec2 = 8 //16 //16
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
		Str("channel_type", "voice").
		Int("decimation_1", dec1).
		Int("decimation_2", dec2).
		Int("intermediate_freq_1", int(if1)).
		Int("intermediate_freq_2", int(if2)).
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
	fa := float64(4000)
	fb := float64(2000)

	lpfCoeffs := fir.MakeLowPass(1.0, if1, fa, fb, fir.Hamming)
	freq.proc.AddBlock(processor.NewDSPWorkerCC(
		"lowpass_decimator",
		"Lowpass Decimator",
		int(if1),
		int(if2),
		dsp.MakeDecimationFirFilter(dec2, lpfCoeffs),
		processor.ShowFFTBalance(),
	))

	freq.proc.AddBlock(processor.NewDSPWorkerCC(
		"squelch",
		"Squelch",
		int(if2),
		int(if2),
		dsp.MakeSquelch(float32(sys.SquelchLevel), 0.1),
	))

	deviation := 4000

	freq.proc.AddBlock(processor.NewDSPWorkerCF(
		"quad_demod",
		"Quadrature Demodulator",
		int(if2),
		int(if2),
		quad.MakeQuadDemod(float32(if2)/(4*math.Pi*float32(deviation))),
		processor.WithVizLength(int(if2)/40),
	))

	freq.proc.AddBlock(processor.NewDSPWorkerFF(
		"fm_deemphasis",
		"FM Deemphasis",
		int(if2),
		int(if2),
		dsp.MakeFMDeemph(0.000075, float32(if2)),
		processor.WithVizLength(int(if2)/40),
	))

	freq.proc.AddBlock(processor.NewDSPWorkerFF(
		"first_stage",
		"First Stage",
		int(if2),
		int(if2),
		dsp.MakeFloatFirFilter(
			fir.MakeLowPass(1.0, if2, 3000, 200, fir.Hamming),
		),
		processor.WithVizLength(int(if2)/40),
	))

	freq.proc.AddBlock(processor.NewDSPWorkerFF(
		"final_highpass",
		"Final Highpass",
		int(if2),
		int(if2),
		dsp.MakeFloatFirFilter(
			fir.MakeHighPass(1.0, if2, 200, 100, fir.Hamming),
		),
		processor.WithVizLength(t.opts.VoiceOutputSampleRate/40),
		processor.WithPlotType(viz.PlotTypeLines),
	))

	freq.proc.AddBlock(processor.NewDSPWorkerFF(
		"resampler",
		"Audio Resampler",
		int(if2),
		int(t.opts.VoiceOutputSampleRate),
		dsp.MakeFloatResampler(127, float32(t.opts.VoiceOutputSampleRate)/float32(if2)),
		processor.WithVizLength(t.opts.VoiceOutputSampleRate/40),
		processor.WithPlotType(viz.PlotTypeLines),
	))

	freq.proc.AddBlock(processor.NewDSPWorkerFF(
		"final_bandpass",
		"Final Bandpass",
		t.opts.VoiceOutputSampleRate,
		t.opts.VoiceOutputSampleRate,
		dsp.MakeFloatFirFilter(
			fir.MakeBandPass(1.15, float64(t.opts.VoiceOutputSampleRate), 300, 3400, 100, fir.Hamming),
		),
		processor.WithVizLength(t.opts.VoiceOutputSampleRate/40),
		processor.WithPlotType(viz.PlotTypeLines),
	))
}

func (freq *VoiceFrequency) initP25(t *Turbine, sys *internalSystem) {
	const p25IFRate = 24000 // target rate: 4800 sym/s × 5 sps

	freq.proc = processor.NewProcessor(fmt.Sprintf("%d-voice-%d", sys.ID, freq.Frequency), "Radio Input", t.vizServer)

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
		Str("channel_type", "voice").
		Str("system_type", "p25").
		Int("decimation_1", dec1).
		Int("decimation_2", dec2).
		Int("intermediate_freq_1", int(if1)).
		Int("intermediate_freq_2", int(if2)).
		Int("intermediate_rate", p25IFRate).
		Str("shift_freq", op25.MHzToString(shiftFreq)).
		Str("bfo_freq", op25.MHzToString(int(if1*bfoFreq))).
		Msg("initializing P25 voice channel")

	bpfCoeffs := fir.MakeComplexBandPass(1.0,
		float64(t.opts.SampleRate),
		float64(shiftFreq)-if1/2.0,
		float64(shiftFreq)+if1/2.0,
		if1/2,
		fir.Hamming,
	)

	freq.proc.AddBlock(processor.NewDSPWorkerCC(
		"bandpass_decimator",
		"Bandpass Decimator",
		t.opts.SampleRate,
		int(if1),
		dsp.MakeDecimationCTFirFilter(dec1, bpfCoeffs),
	))

	freq.proc.AddBlock(processor.NewDSPWorkerCC(
		"bfo_mixer",
		"BFO Mixer",
		int(if1),
		int(if1),
		mixer.NewWaveformMixer(int(if1), int(if1*bfoFreq)),
	))

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
		p25IFRate,
		dsp.MakeRationalResampler(p25IFRate/1000, int(if2)/1000),
	))

	fa = 6250
	fb = fa + 625

	cutoffLpfCoeffs := fir.MakeLowPass(1.0,
		float64(p25IFRate),
		(fb+fa)/2,
		fb-fa,
		fir.Hann)

	freq.proc.AddBlock(processor.NewDSPWorkerCC(
		"cutoff",
		"Cutoff Filter",
		p25IFRate,
		p25IFRate,
		dsp.MakeFirFilter(cutoffLpfCoeffs),
	))

	freq.proc.AddBlock(processor.NewDSPWorkerCF(
		"quad_demod",
		"FM Demodulation",
		p25IFRate,
		p25IFRate,
		quad.MakeQuadDemod(
			float32(p25IFRate)/(2*math.Pi*float32(p25.P25SymbolRate)),
		)))

	freq.proc.AddBlock(processor.NewDSPWorkerFF(
		"baseband_amp",
		"Baseband Amp (RMS AGC)",
		p25IFRate,
		p25IFRate,
		rmsagc.NewRMSAGC(0.01, 0.61)))

	sps := p25IFRate / p25.P25SymbolRate
	ntaps := (7 * sps) | 1
	symbolFilterTaps := dsp.MakeRRC(1.0, float64(p25IFRate), float64(p25.P25SymbolRate), 0.35, ntaps)

	freq.proc.AddBlock(processor.NewDSPWorkerFF(
		"symbol_filter",
		"Symbol Filter (RRC)",
		p25IFRate,
		p25IFRate,
		dsp.MakeFloatFirFilter(symbolFilterTaps)))

	freq.proc.AddBlock(processor.NewDSPWorkerFF(
		"fsk_demodulator",
		"FSK Demodulator (4FSK)",
		p25IFRate,
		p25.P25SymbolRate,
		fsk4.NewFSK4Demodulator(p25IFRate, p25.P25SymbolRate, false),
		processor.WithVizLength(26),
	))

	freq.proc.AddBlock(processor.NewDSPWorkerFB(
		"dibit_slicer",
		"Dibit Slicer",
		p25.P25SymbolRate,
		p25.P25SymbolRate,
		slicer.NewDibitSlicer()))

	// Create vocoder
	voc, err := vocoder.NewMBELibDecoder()
	if err != nil {
		t.logger.Error().Err(err).Msg("failed to create IMBE vocoder")
		return
	}

	freq.voiceDecoder = framep25.NewVoiceAssembler(
		voc,
		t.outputChan,
		freq.SystemID,
		t.opts.VoiceOutputSampleRate,
		t.logger,
	)
	freq.voiceDecoder.SetFrequency(freq.Frequency)
}

func (freq *VoiceFrequency) init(t *Turbine, sys *internalSystem) {
	switch sys.SystemType {
	case op25.SystemTypeSmartnet:
		freq.initNBFM(t, sys)
	case op25.SystemTypeP25:
		freq.initP25(t, sys)
	default:
		panic(fmt.Errorf("unknown system type %s", sys.SystemType))
	}
}

func (t *Turbine) processVoiceChannel(ctx context.Context, buf *types.SegmentComplex64, freq *VoiceFrequency) error {
	start := time.Now()
	metrics := map[string]interface{}{
		"sample_length": len(buf.Data),
		"sample_bytes":  len(buf.Data) * 8,
	}

	defer func() {

		metrics["duration"] = time.Since(start).Microseconds()

		go t.writeAPI.WritePoint(influxdb2.NewPoint("voice.processed",
			map[string]string{
				"frequency":    op25.MHzToString(freq.Frequency),
				"sample_type":  "complex64",
				"channel_type": "voice",
			},
			metrics, time.Now()))
	}()

	if freq.voiceDecoder != nil {
		// P25 digital voice path: Complex → Binary (dibits) → voice decoder
		sliced, err := freq.proc.ProcessComplexToBinary(buf, metrics)
		if err != nil {
			return err
		}

		freq.voiceDecoder.Receive(sliced.Data)
		// voiceDecoder internally pushes audio to outputChan
		return nil
	}

	// Existing analog FM path (SmartNet)
	samples, err := freq.proc.ProcessComplexToFloat(buf, metrics)
	if err != nil {
		return err
	}
	samples.Frequency = freq.Frequency

	select {
	case t.outputChan <- &types.TaggedAudioSampleFloat32{
		TalkGroup: &types.TalkGroup{
			SystemID: freq.SystemID,
		},
		Audio: samples,
	}:
	case <-ctx.Done():
		return ctx.Err()
	}

	return nil
}
