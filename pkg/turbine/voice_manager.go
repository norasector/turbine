package turbine

import (
	"sync"
	"time"

	"github.com/norasector/turbine-common/types"
)

type SystemManager struct {
	VMs map[int]*VoiceManager
	mu  sync.Mutex
}

func NewSystemManager() *SystemManager {
	return &SystemManager{
		VMs: make(map[int]*VoiceManager),
	}
}

func (s *SystemManager) VMForSystemID(systemID int) *VoiceManager {
	if systemID == 0 {
		panic("got 0 system ID")
	}
	s.mu.Lock()
	vm, ok := s.VMs[systemID]
	if !ok {
		vm = NewVoiceManager(systemID)
		s.VMs[systemID] = vm
	}
	s.mu.Unlock()
	return vm
}

type VoiceManager struct {
	talkGroupsByFreq     map[int]types.TalkGroup
	talkGroupsByTGID     map[int]types.TalkGroup
	talkGroupsBySourceID map[int]types.TalkGroup
	mu                   sync.RWMutex
	purgeTime            time.Duration
	systemID             int
}

func NewVoiceManager(systemID int) *VoiceManager {
	return &VoiceManager{
		talkGroupsByFreq:     make(map[int]types.TalkGroup),
		talkGroupsByTGID:     make(map[int]types.TalkGroup),
		talkGroupsBySourceID: make(map[int]types.TalkGroup),
		systemID:             systemID,
		purgeTime:            time.Second * 3,
	}
}

func (v *VoiceManager) validateReturn(tg *types.TalkGroup) *types.TalkGroup {
	if tg == nil {
		return nil
	}
	if time.Since(tg.LastUpdate) > v.purgeTime || tg.Frequency == 0 {
		return nil
	}

	// Return a copy of the talk group
	copy := *tg
	return &copy
}

func (v *VoiceManager) TalkGroupForFrequency(freq int) *types.TalkGroup {
	v.mu.RLock()
	tg := v.talkGroupsByFreq[freq]
	v.mu.RUnlock()
	return v.validateReturn(&tg)
}

func (v *VoiceManager) TalkGroupForID(id int) *types.TalkGroup {
	v.mu.RLock()
	tg := v.talkGroupsByTGID[id]
	v.mu.RUnlock()
	return v.validateReturn(&tg)
}

func (v *VoiceManager) TalkGroupForSourceID(sid int) *types.TalkGroup {
	v.mu.RLock()
	tg := v.talkGroupsBySourceID[sid]
	v.mu.RUnlock()
	return v.validateReturn(&tg)
}

func (v *VoiceManager) UpdateGroup(tgid, sourceID, freq int) {
	v.mu.Lock()
	tg, ok := v.talkGroupsByTGID[tgid]
	if !ok {
		tg = types.TalkGroup{}
	} else {
		oldFreq := tg.Frequency
		oldSourceID := tg.SourceID
		if freq != oldFreq {
			delete(v.talkGroupsByFreq, oldFreq)
		}
		if sourceID != oldSourceID {
			delete(v.talkGroupsBySourceID, oldSourceID)
		}
	}

	tg.ID = tgid
	tg.SystemID = v.systemID
	tg.SourceID = sourceID
	tg.Frequency = freq
	tg.LastUpdate = time.Now()
	v.talkGroupsByFreq[freq] = tg
	v.talkGroupsByTGID[tgid] = tg
	v.talkGroupsBySourceID[sourceID] = tg
	v.mu.Unlock()
}
