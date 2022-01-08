package turbine

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	influxdb2 "github.com/influxdata/influxdb-client-go"
	"github.com/influxdata/influxdb-client-go/api"
	"github.com/norasector/turbine-common/types"
	"github.com/norasector/turbine/pkg/dsp/viz"
	"github.com/norasector/turbine/pkg/op25"
	"github.com/norasector/turbine/pkg/turbine/device"
	"github.com/norasector/turbine/pkg/util"
	"golang.org/x/sync/errgroup"
)

type Turbine struct {
	device           device.Device
	opts             Options
	writeAPI         api.WriteAPI
	rawSampleChan    chan *types.SegmentComplex64
	outputChan       chan *types.TaggedAudioSampleFloat32
	updateChan       chan op25.DataPacket
	output           io.Writer
	vizServer        *viz.Server
	sm               *SystemManager
	controlFreqs     []*ControlFrequency
	voiceFreqs       []*VoiceFrequency
	voiceFreqCache   map[int]struct{}
	controlFreqCache map[int]struct{}
	logger           zerolog.Logger
	systemMap        map[int]*internalSystem

	mu        sync.RWMutex
	controlMu sync.RWMutex
	cancel    context.CancelFunc
	ctx       context.Context
}

type TurbineOption func(t *Turbine) error

func WithInfluxDB(influxClient api.WriteAPI) TurbineOption {
	return func(t *Turbine) error {
		t.writeAPI = influxClient
		return nil
	}
}

func WithImageServer(vizServer *viz.Server) TurbineOption {
	return func(t *Turbine) error {
		t.vizServer = vizServer
		return nil
	}
}

func WithLogger(logger zerolog.Logger) TurbineOption {
	return func(t *Turbine) error {
		t.logger = logger
		return nil
	}
}

func NewTurbine(device device.Device, options Options, opts ...TurbineOption) (*Turbine, error) {
	t := &Turbine{
		device:           device,
		opts:             options,
		output:           os.Stdout,
		rawSampleChan:    make(chan *types.SegmentComplex64, 1),
		outputChan:       make(chan *types.TaggedAudioSampleFloat32),
		updateChan:       make(chan op25.DataPacket, 32),
		writeAPI:         &util.MockWriteAPI{}, // overwritten with option
		sm:               NewSystemManager(),
		voiceFreqCache:   make(map[int]struct{}),
		controlFreqCache: make(map[int]struct{}),
		systemMap:        make(map[int]*internalSystem),
		logger:           log.Logger,
	}

	for _, sys := range options.Systems {
		t.systemMap[sys.ID] = &internalSystem{
			System: sys,
		}
	}

	for _, opt := range opts {
		if err := opt(t); err != nil {
			return nil, err
		}
	}

	if t.opts.CenterFreq == 0 || t.opts.SampleRate == 0.0 || t.opts.VoiceOutputSampleRate == 0.0 {
		return nil, fmt.Errorf("must specify center freq, sample rate, and output rate")
	}

	return t, nil
}

func (t *Turbine) Stop() error {
	t.cancel()
	if t.vizServer != nil {
		t.vizServer.Stop(context.TODO())
	}
	err := t.device.Stop()
	return err
}

func (t *Turbine) Start(ctx context.Context) error {
	eg, ctx := errgroup.WithContext(ctx)
	t.ctx, t.cancel = context.WithCancel(ctx)

	if t.opts.SampleRate > t.device.MaxSampleRate() {
		return fmt.Errorf("error: sample rate %d > device max sample rate %d", t.opts.SampleRate, t.device.MaxSampleRate())
	}

	for _, sys := range t.systemMap {
		sys.dataPacketChan = make(chan op25.OSWPacket)
		for _, freq := range sys.ControlFrequencies {
			ch := NewControlFrequency(t, sys, freq)
			t.controlFreqs = append(t.controlFreqs, ch)
			t.controlFreqCache[ch.Frequency] = struct{}{}
		}
	}

	eg.Go(func() error {
		return t.device.Start(ctx,
			t.opts.CenterFreq,
			t.opts.SampleRate,
			t.rawSampleChan)
	})

	if t.vizServer != nil {
		eg.Go(func() error {
			return t.vizServer.Run(ctx)
		})
	}
	eg.Go(t.processDataPackets)

	for i := 0; i < runtime.NumCPU(); i++ {
		eg.Go(t.outputSamples)
	}

	eg.Go(t.processRawSamples)

	for _, output := range t.opts.AudioOutputs {
		thisOutput := output
		eg.Go(func() error {
			return thisOutput.Start(t.ctx)
		})
	}

	log.Info().
		Str("center_freq", op25.MHzToString(t.opts.CenterFreq)).
		Str("sample_rate", op25.MHzToString(t.opts.SampleRate)).
		Msg("Starting")

	if err := eg.Wait(); err != nil {
		return err
	}
	return nil
}

func (t *Turbine) outputSamples() error {
	for {
		select {
		case <-t.ctx.Done():
			return t.ctx.Err()
		case buf := <-t.outputChan:
			// We know systemID but need the rest.
			tg := t.sm.VMForSystemID(buf.TalkGroup.SystemID).TalkGroupForFrequency(buf.Audio.Frequency)
			if tg == nil {
				tg = &types.TalkGroup{}
			}

			if tg.ID == 0 {
				continue
			}

			buf.TalkGroup = tg

			skippedOutputs := 0
			for _, output := range t.opts.AudioOutputs {
				select {
				case output.Receive() <- buf:
					// We will not wait on blocked channels.
				default:
					skippedOutputs++
				}
			}

			go t.writeAPI.WritePoint(influxdb2.NewPoint("voice.types.output",
				map[string]string{
					"frequency": op25.MHzToString(buf.Audio.Frequency),
				},
				map[string]interface{}{
					"samples_written": len(buf.Audio.Data),
					"bytes_written":   len(buf.Audio.Data) * 4,
					"skipped_outputs": skippedOutputs,
				}, time.Now()))

		}
	}
}

// func (t *Turbine) reapStaleFrequencies() error {
// 	for {
// 		select {
// 		case <-t.ctx.Done():
// 			return t.ctx.Err()
// 		case <-time.After(time.Minute):
// 			t.mu.Lock()
// 			newFreqs := make([]*VoiceFrequency, 0, len(t.opts.VoiceFrequencies))
// 			for _, freq := range t.opts.VoiceFrequencies {
// 				if time.Since(freq.LastSeen) < t.opts.FrequencyTimeout {
// 					newFreqs = append(newFreqs, freq)
// 				}
// 			}
// 			t.opts.VoiceFrequencies = newFreqs
// 			t.mu.Unlock()
// 		}
// 	}
// }

func (t *Turbine) processRawSamples() error {
	segNum := 0
	for {
		select {
		case <-t.ctx.Done():
			return t.ctx.Err()
		case buf := <-t.rawSampleChan:
			segNum++
			buf.SegmentNumber = segNum

			eg, ctx := errgroup.WithContext(t.ctx)

			t.controlMu.RLock()
			for _, freq := range t.controlFreqs {
				thisFreq := freq
				eg.Go(func() error {
					err := t.processControlChannel(ctx, buf, thisFreq)
					return err
				})
			}
			t.controlMu.RUnlock()

			t.mu.RLock()
			for _, freq := range t.voiceFreqs {
				thisFreq := freq
				eg.Go(func() error {
					err := t.processVoiceChannel(ctx, buf, thisFreq)
					return err
				})
			}
			t.mu.RUnlock()

			if err := eg.Wait(); err != nil {
				return err
			}

			// EXPERIMENTAL.
			// If we disable Go GC we manually call GC here, after we've processed everything.
			// We only do this ~once per second.
			// This increases memory usage significantly but reduces the amount of time spent in GC
			// by a large margin.
			// We choose to do it here because we know we have some time between now and the next
			// buffer coming from the SDR.
			if os.Getenv("GOGC") == "off" && segNum%60 == 0 {
				runtime.GC()
			}
		}
	}
}
