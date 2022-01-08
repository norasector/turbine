package turbine

import (
	"fmt"

	"github.com/norasector/turbine/pkg/op25"
	"github.com/norasector/turbine/pkg/op25/frame"
	"github.com/norasector/turbine/pkg/op25/frame/smartnet"
	"golang.org/x/sync/errgroup"
)

func (t *Turbine) processDataPackets() error {

	eg, ctx := errgroup.WithContext(t.ctx)

	for _, sys := range t.systemMap {

		var proc frame.Processor
		switch sys.SystemType {
		case op25.SystemTypeSmartnet:
			proc = smartnet.NewProcessor(sys.ID, sys.dataPacketChan, t.updateChan, t.writeAPI, t.logger)

		default:
			return fmt.Errorf("unrecognized system: %s", sys.SystemType)
		}

		eg.Go(func() error {
			return proc.Start(ctx)
		})
	}

	eg.Go(func() error {
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case update := <-t.updateChan:

				if update.DestTGID > 0 {
					go t.appendVoiceFrequency(update.SystemID, update.TargetFreq)
					go t.sm.VMForSystemID(update.SystemID).UpdateGroup(int(update.DestTGID), int(update.SrcID), update.TargetFreq)
				} else {
					go t.appendControlFrequency(update.SystemID, update.TargetFreq)
				}
			}
		}
	})

	return eg.Wait()
}

func (t *Turbine) freqWithinBounds(freq int) bool {
	halfBw := t.opts.SampleRate/2 - 25000 // leave enough room at either tail
	min := t.opts.CenterFreq - halfBw
	max := t.opts.CenterFreq + halfBw
	return freq >= min && freq <= max
}

func (t *Turbine) appendVoiceFrequency(systemID, freq int) {
	t.mu.Lock()
	if _, ok := t.voiceFreqCache[freq]; !ok && t.freqWithinBounds(freq) {
		sys := t.systemMap[systemID]
		if sys == nil {
			panic("could not find system")
		}

		ch := &VoiceFrequency{
			Frequency: freq,
			Bandwidth: sys.VoiceBandwidth,
			SystemID:  systemID,
		}
		ch.init(t, sys)
		t.voiceFreqs = append(t.voiceFreqs, ch)
		t.voiceFreqCache[freq] = struct{}{}
	}
	t.mu.Unlock()
}

func (t *Turbine) appendControlFrequency(systemID, freq int) {
	t.controlMu.Lock()
	if _, ok := t.controlFreqCache[freq]; !ok && t.freqWithinBounds(freq) {
		t.logger.Debug().Str("freq", op25.MHzToString(freq)).Msg("got new control freq")

		sys := t.systemMap[systemID]
		if sys == nil {
			panic("could not find system")
		}

		ch := NewControlFrequency(t, sys, freq)

		t.controlFreqs = append(t.controlFreqs, ch)
		t.controlFreqCache[freq] = struct{}{}
	}
	t.controlMu.Unlock()
}
