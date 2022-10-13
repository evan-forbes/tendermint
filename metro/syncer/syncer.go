package syncer

import (
	"context"
	"fmt"

	"github.com/tendermint/tendermint/libs/log"

	"github.com/tendermint/tendermint/metro/execute"
	"github.com/tendermint/tendermint/metro/retriever"
	"github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/store"
)

// Syncer handles long-term catchup syncing.
type Syncer struct {
	// immutable
	initialState state.State
	logger       log.Logger
	blockExec    *execute.BlockExecutor
	store        *store.BlockStore

	ret *retriever.Retriever
}

// NewBlockchainReactor returns new reactor instance.
func New(
	state state.State,
	blockExec *execute.BlockExecutor,
	store *store.BlockStore,
	ret *retriever.Retriever,
	logger log.Logger,
) *Syncer {
	if state.LastBlockHeight != store.Height() {
		panic(fmt.Sprintf("state (%v) and store (%v) height mismatch", state.LastBlockHeight,
			store.Height()))
	}

	bcR := &Syncer{
		initialState: state,
		blockExec:    blockExec,
		store:        store,
		ret:          ret,
		logger:       logger,
	}
	return bcR
}

func (snc *Syncer) SwitchToFastSync(s state.State) error {
	snc.initialState = s
	return nil
}

func (snc *Syncer) Start(ctx context.Context) error {
	snc.ret.Start(ctx)
	return nil
}

func (snc *Syncer) Stop() {
	snc.ret.Stop()
}
