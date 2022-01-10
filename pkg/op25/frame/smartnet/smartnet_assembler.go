package smartnet

import (
	"context"
	"time"

	"github.com/norasector/turbine/pkg/op25"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	SmartnetMagicNumber   uint8  = 0xAC
	SmartnetSyncLength           = 8
	SmartnetFrameLength          = 84
	SmartnetPayloadLength        = 76
	SmartnetDataLength           = 27
	SmartnetCRCLength            = 10
	SmartnetIDXOr         uint16 = 0x33C7
	SmartnetCmdXOr        uint16 = 0x032A
	SmartnetIDInvXOr      uint16 = (^SmartnetIDXOr) & 0xffff
	SmartnetCmdInvXOr     uint16 = ^SmartnetCmdXOr & 0x3ff
)

type SmartnetPacket struct {
	Address uint16
	Group   uint8
	Command uint16
	RawData [6]byte
}

type SmartnetAssembler struct {
	systemID int
	buf      [2 * SmartnetFrameLength]byte
	rawFrame [SmartnetPayloadLength]byte
	eccFrame [SmartnetDataLength + SmartnetCRCLength]byte
	// packet      SmartnetPacket
	bufIdx      uint16
	symbolCount uint
	rxCount     int
	syncReg     byte
	inSync      bool
	timer       *time.Timer
	outputChan  chan op25.OSWPacket
	logger      zerolog.Logger
	ctx         context.Context
}

func NewSmartnetAssembler(ctx context.Context, systemID int, ch chan op25.OSWPacket, logger zerolog.Logger) *SmartnetAssembler {
	return &SmartnetAssembler{
		timer:      time.NewTimer(time.Second),
		outputChan: ch,
		ctx:        ctx,
		logger:     logger,
		systemID:   systemID,
	}
}

func (s *SmartnetAssembler) insertSymbol(b byte) {
	s.buf[s.bufIdx] = b
	s.buf[s.bufIdx+SmartnetFrameLength] = b
	s.bufIdx = (s.bufIdx + 1) % SmartnetFrameLength
}

func (s *SmartnetAssembler) receiveSymbol(symbol byte) {
	syncDetected := false
	s.symbolCount++
	s.syncReg = ((s.syncReg << 1) & 0xff) | (symbol & 1)
	if s.syncReg^SmartnetMagicNumber == 0 {
		syncDetected = true
	}
	s.insertSymbol(symbol)
	s.rxCount++

	select {
	case <-s.timer.C:
		log.Debug().Str("system", "smartnet").Msg("sync timer expired")
		s.inSync = false
		s.rxCount = 0
		s.timer.Reset(time.Second)
		return
	default:
	}

	if syncDetected && !s.inSync {
		s.inSync = true
		s.rxCount = 0
		return
	}

	if !s.inSync || (s.inSync && (s.rxCount < SmartnetFrameLength)) {
		return
	}

	if !syncDetected {
		log.Debug().Str("system", "smartnet").Msg("smartnet sync lost")
		s.inSync = false
		s.rxCount = 0
		return
	}

	s.rxCount = 0

	s.deinterleave(s.buf[s.bufIdx : s.bufIdx+SmartnetPayloadLength])
	s.errorCorrection()

	crcOK, packet := s.crcCheck()
	if !crcOK {
		log.Debug().Str("system", "smartnet").Msg("smartnet CRC failure")
		return
	} else {
		select {
		case <-s.ctx.Done():
			return
		case s.outputChan <- op25.OSWPacket{
			SystemID:   s.systemID,
			SystemType: op25.SystemTypeSmartnet,
			Packet:     packet,
			Timestamp:  time.Now().UTC()}:

			// if packet.Address%2 == 1 {
			// 	var computed uint16 = uint16(packet.RawData[0])<<8 + uint16(packet.RawData[1])
			// 	log.Info().Uint16("address", packet.Address).Uint16("computed", computed).Msg("found item")
			// }

		}
	}
	s.timer.Reset(time.Second)
}

func (s *SmartnetAssembler) crcCheck() (bool, SmartnetPacket) {
	var crcaccum uint16 = 0x0393
	var crcop uint16 = 0x036e
	var crcgiven uint16

	for j := 0; j < SmartnetDataLength; j++ {
		if crcop&0x01 == 1 {
			crcop = (crcop >> 1) ^ 0x0225
		} else {
			crcop >>= 1
		}

		if s.eccFrame[j]&0x01 > 0 {
			crcaccum = crcaccum ^ crcop
		}
	}

	crcgiven = 0x0000
	for j := 0; j < SmartnetCRCLength; j++ {
		crcgiven <<= 1
		var frameVal byte = s.eccFrame[j+SmartnetDataLength]
		crcgiven += uint16(^frameVal) & 0x01
	}

	if crcgiven == crcaccum {
		var packet SmartnetPacket
		for j := 0; j < 16; j++ {
			packet.Address = (packet.Address << 1) + uint16(s.eccFrame[j])&0x1
		}
		packet.Address ^= SmartnetIDInvXOr

		packet.Group = ^s.eccFrame[16] & 0x1
		packet.Command = 0
		for j := 17; j < 27; j++ {
			packet.Command = (packet.Command << 1) + uint16(s.eccFrame[j])&0x1
		}
		packet.Command ^= SmartnetCmdInvXOr

		packet.RawData[0] = byte(packet.Address >> 8)
		packet.RawData[1] = byte(packet.Address & 0xff)
		packet.RawData[2] = packet.Group
		packet.RawData[3] = byte(packet.Command >> 8)
		packet.RawData[4] = byte(packet.Command & 0xff)
		packet.RawData[5] = 0

		// if packet.Address == 37331 {
		// 	fmt.Println("37331", s.eccFrame[:16])
		// } else if packet.Address == 60336 {
		// 	fmt.Println("60336", s.eccFrame[:16])
		// }

		return true, packet
	}

	return false, SmartnetPacket{}

}

func (s *SmartnetAssembler) deinterleave(buf []byte) {
	for k := 0; k < SmartnetPayloadLength/4; k++ {
		for l := 0; l < 4; l++ {
			s.rawFrame[k*4+l] = buf[k+l*19]
		}
	}
}

func (s *SmartnetAssembler) errorCorrection() {
	var expected [SmartnetPayloadLength]byte
	var syndrome [SmartnetPayloadLength]byte

	expected[0] = s.rawFrame[0] & 0x01
	expected[1] = s.rawFrame[0] & 0x01

	for k := 2; k < SmartnetPayloadLength; k += 2 {
		expected[k] = s.rawFrame[k] & 0x01
		expected[k+1] = (s.rawFrame[k] & 0x01) ^ (s.rawFrame[k-2] & 0x01)
	}

	for k := 0; k < SmartnetPayloadLength; k++ {
		syndrome[k] = expected[k] ^ (s.rawFrame[k] & 0x01)
	}

	for k := 0; k < (SmartnetPayloadLength/2)-1; k++ {
		if syndrome[2*k+1] == 1 && syndrome[2*k+3] == 1 {
			s.eccFrame[k] = (^s.rawFrame[2*k]) & 0x01
		} else {
			s.eccFrame[k] = s.rawFrame[2*k]
		}
	}
}

func (s *SmartnetAssembler) Receive(buf []byte) {
	for i := 0; i < len(buf); i++ {
		s.receiveSymbol(buf[i])
	}
}
