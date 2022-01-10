package smartnet

import (
	"context"
	"fmt"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go"
	"github.com/influxdata/influxdb-client-go/api"
	"github.com/norasector/turbine/pkg/op25"
	"github.com/rs/zerolog"
)

type SmartnetProcessor struct {
	smartnetPacketBuffer []parsedSmartnetPacket
	dataPacketChan       chan op25.OSWPacket
	updateChan           chan op25.DataPacket
	logger               zerolog.Logger
	writeAPI             api.WriteAPI
	systemID             int
}

func NewProcessor(systemID int, dataPacketChan chan op25.OSWPacket, updateChan chan op25.DataPacket, writeAPI api.WriteAPI, logger zerolog.Logger) *SmartnetProcessor {
	return &SmartnetProcessor{
		dataPacketChan: dataPacketChan,
		updateChan:     updateChan,
		writeAPI:       writeAPI,
		systemID:       systemID,
		logger:         logger,
	}
}

func (s *SmartnetProcessor) popSmartnetPacket() parsedSmartnetPacket {
	pkt := s.smartnetPacketBuffer[0]
	s.smartnetPacketBuffer = s.smartnetPacketBuffer[1:]
	return pkt
}

func (s *SmartnetProcessor) pushLeftSmartnetPacket(packet parsedSmartnetPacket) {
	s.smartnetPacketBuffer = append([]parsedSmartnetPacket{packet}, s.smartnetPacketBuffer...)
}

type parsedSmartnetPacket struct {
	SmartnetPacket
	ts            time.Time
	isChannel     bool
	frequency     int
	isControlFreq bool
}

func (s *SmartnetProcessor) Start(ctx context.Context) error {

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case oswPacket := <-s.dataPacketChan:
			switch packet := oswPacket.Packet.(type) {
			case SmartnetPacket:

				parsed := parsedSmartnetPacket{
					ts:             oswPacket.Timestamp,
					SmartnetPacket: packet,
				}
				metrics := make(map[string]interface{})

				parseSmartnetPacket(&parsed)

				s.smartnetPacketBuffer = append(s.smartnetPacketBuffer, parsed)

				if err := s.processSmartnetPacket(metrics); err != nil {
					return err
				}

				if len(metrics) > 0 {
					go s.writeAPI.WritePoint(influxdb2.NewPoint("smartnet.packet.processed",
						map[string]string{
							"type": "smartnet",
						},
						metrics, time.Now()))

				}
			default:
				return fmt.Errorf("unrecognized packet type %s", oswPacket.SystemType)
			}
		}
	}
}

func parseSmartnetPacket(packet *parsedSmartnetPacket) {
	packet.isChannel = func() bool {
		cmd := packet.Command
		// if band == "800"
		if cmd > 0x22f { // && subtype=reband
			return false
		}

		if cmd <= 0x2f7 ||
			(cmd >= 0x32f && cmd <= 0x33f) ||
			(cmd >= 0x3c1 && cmd <= 0x3fe) ||
			cmd == 0x3be {
			return true
		}

		return false
	}()

	if packet.isChannel {
		packet.frequency = smartnetFrequency(packet.Command)
	}
}

func smartnetFrequency(cmd uint16) int {
	var freq int
	iCmd := int(cmd)
	// this is 800 reband ONLY
	switch {
	case iCmd <= 0x2cf:

		switch {
		case iCmd < 0x1b8:
			freq = 851.0125e6 + 2.5e4*iCmd
		case iCmd >= 0x1b8 && iCmd <= 0x22f:
			freq = 851.0250e6 + 2.5e4*(iCmd-0x1b8)
		}

	case iCmd <= 0x2f7:
		freq = 866.0000e6 + 2.5e4*(iCmd-0x2d0)
	case iCmd >= 0x32F && iCmd <= 0x33F:
		freq = 867.0000e6 + 2.5e4*(iCmd-0x32f)
	case iCmd == 0x3BE:
		freq = 868.9750e6
	case iCmd >= 0x3C1 && iCmd <= 0x3FE:
		freq = 867.4250e6 + 2.5e4*(iCmd-0x3C1)
	}

	return freq //float32(math.Round(freq*100000) / 100000)
}

func incMap(m map[string]interface{}, key string) {
	val := m[key]
	if v, ok := val.(int); ok {
		m[key] = v + 1
	} else {
		m[key] = 1
	}
}

func (s *SmartnetProcessor) processSmartnetPacket(metrics map[string]interface{}) error {
	for len(s.smartnetPacketBuffer) >= 3 {
		if err := func() error {

			osw2 := s.popSmartnetPacket()
			switch {
			case osw2.Command == 0x308 || osw2.Command == 0x309:
				osw1 := s.popSmartnetPacket()

				switch {

				case osw1.isChannel && osw1.Group > 0 && osw1.Address != 0 && osw2.Address != 0:
					srcID := osw2.Address
					destTGID := osw1.Address & 0xfff0
					targetFreq := osw1.frequency

					s.logger.Debug().
						Int("source_id", int(srcID)).
						Int("tgid", int(destTGID)).
						Str("freq", op25.MHzToString(targetFreq)).
						Str("system", "smartnet").
						Msg("group grant")

					s.updateChan <- op25.DataPacket{
						DestTGID:   destTGID,
						SrcID:      srcID,
						TargetFreq: targetFreq,
						SystemID:   s.systemID,
					}

					incMap(metrics, "group_update")

				case osw1.isChannel && osw1.Group == 0 && osw1.Address&0xFF00 == 0x1f00:
					rxSysID := osw2.Address
					rxCCFreq := osw1.frequency

					s.logger.Debug().
						Int("system_id", int(rxSysID)).
						Str("control_channel", op25.MHzToString(rxCCFreq)).
						Str("system", "smartnet").
						Msg("system id broadcast")

					incMap(metrics, "sys_id_broadcast")

					s.updateChan <- op25.DataPacket{
						TargetFreq: rxCCFreq,
						SystemID:   s.systemID,
					}

				case osw1.Command == 0x30b:

					osw0 := s.popSmartnetPacket()

					if osw0.isChannel && osw0.Address&0xff00 == 0x1f00 && osw1.Address&0xfc00 == 0x2800 && osw1.Address&0x3ff == osw0.Command {
						rxSysID := osw2.Address
						rxCCFreq := osw0.frequency

						s.logger.Debug().
							Int("system_id", int(rxSysID)).
							Str("control_channel", op25.MHzToString(rxCCFreq)).
							Str("system", "smartnet").
							Msg("system id broadcast")

						incMap(metrics, "sys_id_broadcast")

						s.updateChan <- op25.DataPacket{
							TargetFreq: rxCCFreq,
							SystemID:   s.systemID,
						}
					} else {
						s.pushLeftSmartnetPacket(osw0)
						if osw1.Address&0xfc00 == 0x2800 {
							rxSysID := osw2.Address
							rxCCFreq := smartnetFrequency(osw1.Address & 0x3ff)

							s.logger.Debug().
								Int("system_id", int(rxSysID)).
								Str("control_channel", op25.MHzToString(rxCCFreq)).
								Str("system", "smartnet").
								Msg("system id broadcast")

							incMap(metrics, "sys_id_broadcast")

							s.updateChan <- op25.DataPacket{
								TargetFreq: rxCCFreq,
								SystemID:   s.systemID,
							}

						}
					}

				case osw1.Command == 0x310:
					srcID := osw2.Address
					destTGID := osw1.Address & 0xfff0

					s.logger.Debug().
						Int("source_id", int(srcID)).
						Int("tgid", int(destTGID)).
						Str("system", "smartnet").
						Msg("affiliation broadcast")

					incMap(metrics, "affiliation_broadcast")

				case osw1.Command == 0x320:

					osw0 := s.popSmartnetPacket()
					if osw0.Command == 0x30b {
						if osw0.Address&0xfc00 == 0x6000 {
							sysID := osw2.Address
							cellID := (osw1.Address >> 10) & 0x3f
							band := (osw1.Address >> 7) & 0x7
							feat := osw1.Address & 0x3f
							// freq := osw0.frequency

							s.logger.Debug().
								Int("system_id", int(sysID)).
								Int("cell_id", int(cellID)).
								Int("band", int(band)).
								Int("features", int(feat)).
								Str("system", "smartnet").
								Msg("cellsite broadcast")

							incMap(metrics, "cellsite_broadcast")
						}
					} else {
						s.pushLeftSmartnetPacket(osw0)
					}
				default:
					s.pushLeftSmartnetPacket(osw1)
				}

			case osw2.Command == 0x321:
				osw1 := s.popSmartnetPacket()
				if osw1.isChannel && osw1.Group > 0 && osw1.Address > 0 {
					srcID := osw2.Address
					destTGID := osw1.Address & 0xfff0
					targetFreq := osw1.frequency

					s.logger.Debug().
						Int("source_id", int(srcID)).
						Int("tgid", int(destTGID)).
						Str("frequency", op25.MHzToString(targetFreq)).
						Str("system", "smartnet").
						Msg("astro grant")

					incMap(metrics, "astro_grant")
					s.updateChan <- op25.DataPacket{
						DestTGID:   destTGID,
						SrcID:      srcID,
						TargetFreq: targetFreq,
						SystemID:   s.systemID,
					}

				} else {
					s.pushLeftSmartnetPacket(osw1)
				}
			case osw2.isChannel && osw2.Group > 0:
				destTGID := osw2.Address & 0xfff0
				targetFreq := osw2.frequency
				// TODO update_vocie_freq

				s.logger.Debug().
					Int("tgid", int(destTGID)).
					Str("frequency", op25.MHzToString(targetFreq)).
					Str("system", "smartnet").
					Msg("group update")

				incMap(metrics, "group_update")

				s.updateChan <- op25.DataPacket{
					DestTGID:   destTGID,
					SrcID:      0,
					TargetFreq: targetFreq,
					SystemID:   s.systemID,
				}

			case osw2.isChannel && osw2.Group == 0 && osw2.Address&0xff00 == 0x1f00:
				s.logger.Debug().
					Str("frequency", op25.MHzToString(osw2.frequency)).
					Str("system", "smartnet").
					Msg("control channel broadcast")
				incMap(metrics, "control_channel_broadcast")

				s.updateChan <- op25.DataPacket{
					TargetFreq: osw2.frequency,
					SystemID:   s.systemID,
				}
			default:
				incMap(metrics, "unknown")
			}
			return nil
		}(); err != nil {
			return err
		}
	}

	return nil
}
