package retriever

import (
	"context"
	"sync"
	"time"

	"github.com/tendermint/tendermint/libs/log"

	"github.com/tendermint/tendermint/metro/da"
	"github.com/tendermint/tendermint/types"
)

type Config struct {
	Namespace     []byte `json:"namespace"`
	DAStartHeight int64
}

type Retriever struct {
	dalc   da.BlockRetriever
	logger log.Logger
	cancel context.CancelFunc

	Blocks chan *types.Block

	mut           *sync.Mutex
	dALayerHeight int64
}

func NewRetriever(cfg Config, dalc da.BlockRetriever, logger log.Logger) *Retriever {
	blocks := make(chan *types.Block, 1000)

	return &Retriever{
		Blocks:        blocks,
		mut:           &sync.Mutex{},
		dalc:          dalc,
		dALayerHeight: cfg.DAStartHeight,
		logger:        logger,
	}
}

func (r *Retriever) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	r.cancel = cancel

	go r.sync(ctx)
}

func (f *Retriever) Stop() {
	f.cancel()
}

func (r *Retriever) sync(ctx context.Context) {
	defer close(r.Blocks)
	for {
		select {
		case <-ctx.Done():
			return
		default:
			for _, b := range r.fetchNextBlocks(time.Second) {
				r.Blocks <- b
			}
			r.mut.Lock()
			r.dALayerHeight++
			r.mut.Unlock()

		}
	}
}

func (r *Retriever) fetchNextBlocks(retryTime time.Duration) []*types.Block {
	res := r.dalc.RetrieveBlocks(uint64(r.dALayerHeight))
	if res.Code != da.StatusSuccess {
		r.logger.Error("failure to retrieve block", "err", res.Message)
		time.Sleep(retryTime)
		return r.fetchNextBlocks(retryTime + time.Second)
	}
	return res.Blocks
}
