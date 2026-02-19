package p25

import (
	"context"
	"fmt"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go"
	"github.com/influxdata/influxdb-client-go/api"
	"github.com/norasector/turbine/pkg/op25"
	p25 "github.com/norasector/turbine/pkg/op25/p25"
	"github.com/rs/zerolog"
)

// P25Processor implements frame.Processor for P25 trunking.
// It reads OSWPackets containing P25ControlPacket (TSBK) data,
// decodes TSBK opcodes, and emits DataPackets for voice channel grants.
type P25Processor struct {
	dataPacketChan chan op25.OSWPacket
	updateChan     chan op25.DataPacket
	logger         zerolog.Logger
	writeAPI       api.WriteAPI
	systemID       int

	// Frequency band identifier table
	idenTable IdenTable
}

// NewProcessor creates a new P25 trunking processor.
func NewProcessor(
	systemID int,
	dataPacketChan chan op25.OSWPacket,
	updateChan chan op25.DataPacket,
	writeAPI api.WriteAPI,
	logger zerolog.Logger,
) *P25Processor {
	return &P25Processor{
		dataPacketChan: dataPacketChan,
		updateChan:     updateChan,
		writeAPI:       writeAPI,
		systemID:       systemID,
		logger:         logger,
		idenTable:      make(IdenTable),
	}
}

// Start implements frame.Processor. Processes P25 TSBK packets.
func (p *P25Processor) Start(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case oswPacket := <-p.dataPacketChan:
			switch pkt := oswPacket.Packet.(type) {
			case P25ControlPacket:
				metrics := make(map[string]interface{})
				p.processTSBK(pkt, metrics)

				if len(metrics) > 0 {
					go p.writeAPI.WritePoint(influxdb2.NewPoint("p25.packet.processed",
						map[string]string{
							"type": "p25",
						},
						metrics, time.Now()))
				}
			default:
				return fmt.Errorf("P25 processor received non-P25 packet: %T", oswPacket.Packet)
			}
		}
	}
}

func (p *P25Processor) processTSBK(pkt P25ControlPacket, metrics map[string]interface{}) {
	tsbk := pkt.TSBK
	if !tsbk.CRCValid {
		incMetric(metrics, "crc_failures")
		return
	}

	switch tsbk.Opcode {
	case p25.TSBKOpcodeGrpVChGrant:
		grant := ParseGrpVChGrant(tsbk)
		freq := p.idenTable.ChannelToFrequency(grant.Channel)

		if freq == 0 {
			p.logger.Debug().
				Uint16("channel", grant.Channel).
				Msg("P25 channel grant: unknown frequency (missing IDEN_UP)")
			incMetric(metrics, "grant_unknown_freq")
			return
		}

		p.logger.Debug().
			Uint16("tgid", grant.TGID).
			Uint32("source_id", grant.SrcID).
			Int("freq", freq).
			Str("system", "p25").
			Msg("group voice channel grant")

		incMetric(metrics, "group_grant")

		p.updateChan <- op25.DataPacket{
			DestTGID:   grant.TGID,
			SrcID:      uint16(grant.SrcID & 0xFFFF),
			TargetFreq: freq,
			SystemID:   p.systemID,
		}

	case p25.TSBKOpcodeGrpVChGrantUpdt:
		updt := ParseGrpVChGrantUpdt(tsbk)

		// First channel/TGID pair
		if updt.Channel1 != 0 && updt.TGID1 != 0 {
			freq1 := p.idenTable.ChannelToFrequency(updt.Channel1)
			if freq1 != 0 {
				p.logger.Debug().
					Uint16("tgid", updt.TGID1).
					Int("freq", freq1).
					Str("system", "p25").
					Msg("group voice channel grant update (1)")

				p.updateChan <- op25.DataPacket{
					DestTGID:   updt.TGID1,
					TargetFreq: freq1,
					SystemID:   p.systemID,
				}
			}
		}

		// Second channel/TGID pair
		if updt.Channel2 != 0 && updt.TGID2 != 0 {
			freq2 := p.idenTable.ChannelToFrequency(updt.Channel2)
			if freq2 != 0 {
				p.logger.Debug().
					Uint16("tgid", updt.TGID2).
					Int("freq", freq2).
					Str("system", "p25").
					Msg("group voice channel grant update (2)")

				p.updateChan <- op25.DataPacket{
					DestTGID:   updt.TGID2,
					TargetFreq: freq2,
					SystemID:   p.systemID,
				}
			}
		}

		incMetric(metrics, "grant_update")

	case p25.TSBKOpcodeIdenUp:
		entry := ParseIdenUp(tsbk)
		p.idenTable[entry.Identifier] = entry

		p.logger.Debug().
			Uint8("iden", entry.Identifier).
			Int64("base_freq", entry.BaseFreq).
			Int("spacing", entry.Spacing).
			Int("bandwidth", entry.Bandwidth).
			Str("system", "p25").
			Msg("identifier update")

		incMetric(metrics, "iden_update")

	case p25.TSBKOpcodeIdenUpVUHP:
		entry := ParseIdenUpVUHP(tsbk)
		p.idenTable[entry.Identifier] = entry

		p.logger.Debug().
			Uint8("iden", entry.Identifier).
			Int64("base_freq", entry.BaseFreq).
			Int("spacing", entry.Spacing).
			Str("system", "p25").
			Msg("identifier update (VU)")

		incMetric(metrics, "iden_update_vu")

	case p25.TSBKOpcodeNetStatBcast:
		p.logger.Debug().Str("system", "p25").Msg("network status broadcast")
		incMetric(metrics, "net_status")

	case p25.TSBKOpcodeRFSSStatBcast:
		p.logger.Debug().Str("system", "p25").Msg("RFSS status broadcast")
		incMetric(metrics, "rfss_status")

	case p25.TSBKOpcodeSysServiceBcast:
		p.logger.Debug().Str("system", "p25").Msg("system service broadcast")
		incMetric(metrics, "sys_service")

	case p25.TSBKOpcodeAdjStStatBcast:
		p.logger.Debug().Str("system", "p25").Msg("adjacent status broadcast")
		incMetric(metrics, "adj_status")

	case p25.TSBKOpcodeGrpAffiliation:
		p.logger.Debug().Str("system", "p25").Msg("group affiliation response")
		incMetric(metrics, "grp_affiliation")

	default:
		p.logger.Trace().
			Uint8("opcode", tsbk.Opcode).
			Str("system", "p25").
			Msg("unhandled TSBK opcode")
		incMetric(metrics, "unknown")
	}
}

func incMetric(m map[string]interface{}, key string) {
	val := m[key]
	if v, ok := val.(int); ok {
		m[key] = v + 1
	} else {
		m[key] = 1
	}
}
