package relayer

import (
	"sync"
)

type state struct {
	mut           *sync.RWMutex
	DALayerHeight int64
	RelayedHeight int64
}

func newState(DALayerHeight, RelayedHeight int64) *state {
	return &state{
		mut:           &sync.RWMutex{},
		DALayerHeight: DALayerHeight,
		RelayedHeight: RelayedHeight,
	}
}

func (s *state) setDALayerHeight(height int64) {
	s.mut.Lock()
	defer s.mut.Unlock()
	s.DALayerHeight = height
}

func (s *state) setRelayedHeight(height int64) {
	s.mut.Lock()
	defer s.mut.Unlock()
	s.RelayedHeight = height
}
