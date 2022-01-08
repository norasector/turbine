package turbine

import (
	"context"

	"github.com/norasector/turbine-common/types"
)

// AudioOutput handles incoming tagged audio samples.
type AudioOutput interface {
	// Start receives a context and should run in a loop, terminating upon ctx closing or on any errors.
	Start(ctx context.Context) error
	// Receive returns a channel that receives tagged audio sample input.
	Receive() chan<- *types.TaggedAudioSampleFloat32
}
