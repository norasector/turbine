package device

import (
	"context"

	"github.com/norasector/turbine-common/types"
)

type Device interface {
	Start(ctx context.Context, centerFreq int, sampleRate int, complexSamples chan *types.SegmentComplex64) error
	Stop() error
	MaxSampleRate() int
}
