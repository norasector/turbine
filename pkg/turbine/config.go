package turbine

import (
	"time"

	"github.com/norasector/turbine/pkg/op25"
	"github.com/norasector/turbine/pkg/turbine/config"
)

type Options struct {
	CenterFreq            int
	SampleRate            int
	VoiceOutputSampleRate int
	Gain                  int
	Squelch               int
	Systems               []config.System
	AudioOutputs          []AudioOutput
	FrequencyTimeout      time.Duration
	RecordLocation        string
	PlaybackLocation      string
}

type internalSystem struct {
	config.System
	dataPacketChan chan op25.OSWPacket
}
