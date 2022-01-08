package op25

import "time"

type SystemType string

const (
	SystemTypeSmartnet SystemType = "smartnet"
)

type OSWPacket struct {
	SystemID   int
	SystemType SystemType
	Packet     interface{}
	Timestamp  time.Time
}

type DataPacket struct {
	DestTGID   uint16
	SrcID      uint16
	TargetFreq int
	SystemID   int
}
