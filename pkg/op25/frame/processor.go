package frame

import "context"

// Processor is a frame processor.  Configuration is dependent on the specific implementation.
// See Smartnet code and control channel code for more idea on how to use.
type Processor interface {
	Start(context.Context) error
}
