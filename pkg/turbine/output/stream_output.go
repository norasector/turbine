package output

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go"
	"github.com/influxdata/influxdb-client-go/api"
	"github.com/norasector/turbine-common/types"
	commonTypes "github.com/norasector/turbine-common/types"
	"github.com/norasector/turbine/pkg/turbine/config"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/proto"
)

const receiveChannels = 8

type TaggedOpusFrameUDPOutput struct {
	dests      []config.OutputDestination
	sampleRate int
	recvChan   chan *types.TaggedAudioSampleFloat32
	opusChan   chan *commonTypes.TaggedAudioFrameOpus
	mu         sync.Mutex
	encoders   map[int]map[int]*OpusEncoder
	metrics    api.WriteAPI
}

type OutputDestination struct {
	Host string
	Port int
}

func NewTaggedOpusFrameUDPOutput(dests []config.OutputDestination, sampleRate int, metrics api.WriteAPI) *TaggedOpusFrameUDPOutput {
	return &TaggedOpusFrameUDPOutput{
		dests:      dests,
		sampleRate: sampleRate,
		recvChan:   make(chan *types.TaggedAudioSampleFloat32, receiveChannels),
		encoders:   make(map[int]map[int]*OpusEncoder),
		opusChan:   make(chan *commonTypes.TaggedAudioFrameOpus),
		metrics:    metrics,
	}
}

func (s *TaggedOpusFrameUDPOutput) Receive() chan<- *types.TaggedAudioSampleFloat32 {
	return s.recvChan
}

func (s *TaggedOpusFrameUDPOutput) getEncoder(systemID, tgid int) (*OpusEncoder, bool, error) {
	s.mu.Lock()
	created := false
	sysEncoders, ok := s.encoders[systemID]
	if !ok {
		sysEncoders = make(map[int]*OpusEncoder)
		s.encoders[systemID] = sysEncoders
	}
	enc, ok := sysEncoders[tgid]
	var err error
	if !ok {
		enc, err = NewOpusEncoder(s.sampleRate, systemID, tgid, s.opusChan)
		if err != nil {
			return nil, false, err
		}
		sysEncoders[tgid] = enc
		created = true
	}
	s.mu.Unlock()

	return enc, created, nil
}

func (s *TaggedOpusFrameUDPOutput) Start(ctx context.Context) error {

	eg, ctx := errgroup.WithContext(ctx)

	const numListeners int = 4

	destAddrs := make([]*net.UDPAddr, 0, len(s.dests))
	for _, dest := range s.dests {

		ips, err := net.LookupIP(dest.Host)
		if err != nil {
			return err
		}
		if len(ips) == 0 {
			return fmt.Errorf("no IPs returned for %s", dest.Host)
		}

		destAddr := &net.UDPAddr{IP: ips[0], Port: dest.Port}
		destAddrs = append(destAddrs, destAddr)
		log.Info().IPAddr("dest_ip", destAddr.IP).Int("port", dest.Port).Msg("stream output starting")
	}

	for i := 0; i < numListeners; i++ {
		eg.Go(func() error {

			conn, err := net.ListenUDP("udp", nil)
			if err != nil {
				return err
			}
			for {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case output := <-s.opusChan:

					pba := output.ToProtobuf()

					encoded, err := proto.Marshal(pba)
					if err != nil {
						log.Warn().Err(err).Msg("error marshaling protobuf")
						continue
					}

					var msgBuf bytes.Buffer
					if err := binary.Write(&msgBuf, binary.LittleEndian, uint16(len(encoded))); err != nil {
						log.Warn().Err(err).Msg("error encoding header size")
						continue
					}
					if _, err := msgBuf.Write(encoded); err != nil {
						log.Warn().Err(err).Msg("error writing encoded message")
						continue
					}

					success := true
					var bytesWritten int
					for _, destAddr := range destAddrs {
						bytesWritten, err = conn.WriteToUDP(msgBuf.Bytes(), destAddr)
						if err != nil {
							log.Error().Err(err).Msg("error writing")
							success = false
						}
					}

					go s.metrics.WritePoint(influxdb2.NewPoint("opus.sent_frame",
						map[string]string{
							"channel_type": "voice",
							"system_id":    strconv.Itoa(output.TalkGroup.SystemID),
							"tgid":         strconv.Itoa(output.TalkGroup.ID),
						},
						map[string]interface{}{
							"bytes_written":  bytesWritten,
							"frame_length":   len(output.Audio.Data),
							"encoded_length": len(encoded),
							"sent": func() int {
								if success {
									return 1
								}
								return 0
							}(),
							"dropped": func() int {
								if success {
									return 0
								}
								return 1
							}(),
						}, time.Now()))
				}
			}
		})
	}

	// Concurrently filter incoming samples to only get what we are looking for.
	for i := 0; i < numListeners; i++ {

		eg.Go(func() error {

			for {
				select {
				case <-ctx.Done():
					return ctx.Err()

				case ts := <-s.recvChan:

					enc, created, err := s.getEncoder(ts.TalkGroup.SystemID, ts.TalkGroup.ID)
					if err != nil {
						return err
					}
					if created {
						eg.Go(func() error {
							return enc.Start(ctx)
						})
					}

					select {
					case <-ctx.Done():
						return ctx.Err()
					case enc.ReceiveChannel() <- ts:
					}

				}
			}
		})
	}

	return eg.Wait()
}
