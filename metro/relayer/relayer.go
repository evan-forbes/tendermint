package relayer

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/metro/da"
	"github.com/tendermint/tendermint/rpc/client/http"

	"github.com/tendermint/tendermint/rpc/client"
	"github.com/tendermint/tendermint/types"
)

type Relayer struct {
	cfg    Config
	client client.Client
	logger log.Logger
	dalc   da.DataAvailabilityLayerClient

	s *state

	cancel context.CancelFunc
}

func NewRelayer(cfg Config, dalc da.DataAvailabilityLayerClient) *Relayer {

	return &Relayer{
		cfg:    cfg,
		logger: log.NewTMLogger(os.Stdout),
		s:      newState(0, 0),
		dalc:   dalc,
	}
}

func (r *Relayer) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	r.cancel = cancel

	err := r.connect(r.cfg.Remote)
	if err != nil {
		return err
	}

	blocks, err := r.subscribeBlocks(ctx)
	if err != nil {
		return err
	}

	go r.postBlocks(blocks)

	return nil
}

func (r *Relayer) connect(ip string) error {
	client, err := http.New(ip, "/websocket")
	if err != nil {
		return err
	}
	client.SetLogger(r.logger)
	err = client.Start()
	if err != nil {
		return err
	}
	r.client = client
	return nil
}

func (r *Relayer) subscribeBlocks(ctx context.Context) (<-chan *types.Block, error) {
	sub, err := r.client.Subscribe(ctx, "relayer", types.QueryForEvent(types.EventNewBlock).String())
	if err != nil {
		return nil, err
	}

	blocks := make(chan *types.Block, 1000)

	go func(blocks chan<- *types.Block) error {
		defer close(blocks)
		for ev := range sub {
			blockRes, ok := ev.Data.(types.EventDataNewBlock)
			if !ok {
				return errors.New("unexpected event")
			}
			blocks <- blockRes.Block
		}
		return nil
	}(blocks)

	return blocks, nil
}

func (r *Relayer) postBlock(b *types.Block, retryDelay time.Duration) {
	res := r.dalc.SubmitBlock(b)
	if res.BaseResult.Code != da.StatusSuccess {
		r.logger.Error("failure to submit block", "err", res.Message)
		time.Sleep(retryDelay)
		r.postBlock(b, retryDelay+time.Second)
	}
	r.s.setRelayedHeight(b.Height)
	r.s.setDALayerHeight(int64(res.DAHeight))
}

func (r *Relayer) postBlocks(blocks <-chan *types.Block) {
	for b := range blocks {
		r.postBlock(b, r.cfg.BlockBatchTimeout)
	}
}
