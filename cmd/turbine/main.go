package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v2"

	influxdb2 "github.com/influxdata/influxdb-client-go"
	"github.com/norasector/turbine/pkg/dsp/viz"
	"github.com/norasector/turbine/pkg/turbine"
	"github.com/norasector/turbine/pkg/turbine/config"
	"github.com/norasector/turbine/pkg/turbine/device"
	"github.com/norasector/turbine/pkg/turbine/device/file"
	hackrfDevice "github.com/norasector/turbine/pkg/turbine/device/hackrf"
	"github.com/norasector/turbine/pkg/turbine/device/rtlsdr"
	"github.com/norasector/turbine/pkg/turbine/output"
	"github.com/samuel/go-hackrf/hackrf"
	"golang.org/x/sync/errgroup"
)

const (
	fileByteReadSize = 262144
	fileReadDelay    = time.Microsecond * 16384
)

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).Level(zerolog.InfoLevel)
	configFile := flag.String("config", "turbine.yaml", "YAML config file")

	flag.Parse()
	if configFile == nil {
		flag.Usage()
		os.Exit(1)
	}

	configContents, err := os.ReadFile(*configFile)
	if err != nil {
		log.Fatal().Err(err).Msg("error reading config file")
	}
	var opts config.Config
	if err := yaml.Unmarshal(configContents, &opts); err != nil {
		log.Fatal().Err(err).Msg("error unmarshaling yaml file")
	}

	// b, _ := json.MarshalIndent(opts, "", "  ")
	// fmt.Println(string(b))

	var device device.Device

	if opts.PlaybackLocation != "" {
		opts.Device = "file"
	}

	switch opts.Device {
	case "rtlsdr":
		log.Info().Str("device", "rlsdr").Msg("initializing device...")
		device, err = rtlsdr.NewRTLSDRDevice(opts.RTLSDRDeviceIndex)
		if err != nil {
			log.Fatal().Str("device", "rlsdr").Err(err).Msg("failed to initialize RTLSDR")
		}
	case "file":
		log.Info().Str("device", "file").Msg("initializing device...")
		// Note that if you read from a file, it expects to be captured from a HackRF -- you will need to modify inputs here to accommodate
		// file dumps from other SDRs
		device, err = file.NewFileDevice(opts.PlaybackLocation, fileByteReadSize, opts.SampleRate, opts.CenterFreq, fileReadDelay)
		if err != nil {
			log.Fatal().Str("device", "rlsdr").Err(err).Msg("failed to init file reader")
		}
		log.Logger = log.Logger.Level(zerolog.DebugLevel)
	default:
		log.Info().Str("device", "hackrf").Msg("initializing device...")
		if err := hackrf.Init(); err != nil {
			log.Fatal().Str("device", "hackrf").Err(err).Msg("failed to initialize hackRF")
		}
		defer hackrf.Exit()

		if opts.RecordLocation != "" {
			device, err = hackrfDevice.NewRecordingHackRFDevice(opts.RecordLocation)
			if err != nil {
				log.Fatal().Str("device", "hackrf").Err(err).Msg("failed to create hackRF recording file device")
			}

		} else {
			device, err = hackrfDevice.NewHackRFDevice()
			if err != nil {
				log.Fatal().Str("device", "hackrf").Err(err).Msg("failed to create hackRF device")
			}
		}
	}

	vizServer := viz.NewServer(opts.VizServer.Port, opts.VizServer.UpdateInterval)
	// vizServer.Enable(false)

	influxWriteAPI := influxdb2.NewClient(opts.InfluxDB.Host, "").WriteAPI(opts.InfluxDB.Organization, opts.InfluxDB.Bucket)

	turbine, err := turbine.NewTurbine(device,
		turbine.Options{
			CenterFreq:            opts.CenterFreq,
			SampleRate:            opts.SampleRate,
			VoiceOutputSampleRate: opts.VoiceSampleOutputRate,
			FrequencyTimeout:      opts.FrequencyTimeout,
			Systems:               opts.Systems,
			AudioOutputs: []turbine.AudioOutput{
				output.NewTaggedOpusFrameUDPOutput(opts.OutputDestinations, opts.VoiceSampleOutputRate, influxWriteAPI),
			},
			RecordLocation:   opts.RecordLocation,
			PlaybackLocation: opts.PlaybackLocation,
		}, turbine.WithInfluxDB(
			influxWriteAPI,
		),
		turbine.WithImageServer(vizServer),
		turbine.WithLogger(log.Logger))
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create receiver")
	}

	eg, ctx := errgroup.WithContext(context.Background())

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	eg.Go(func() error {

		select {
		case <-sigChan:
		case <-ctx.Done():
		}

		return turbine.Stop()
	})

	eg.Go(func() error {
		return turbine.Start(ctx)
	})

	if err := eg.Wait(); err != nil && err != context.Canceled {
		log.Fatal().Err(err).Msg("exited program")
	}
}
