package mock

import (
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/metro/da"
	"github.com/tendermint/tendermint/types"
)

// DataAvailabilityLayerClient is intended only for usage in tests.
// It does actually ensures DA - it stores data in-memory.
type DataAvailabilityLayerClient struct {
	logger   log.Logger
	daHeight uint64
	config   config

	mut    *sync.Mutex
	blocks map[uint64][]*types.Block
}

const defaultBlockTime = 3 * time.Second

type config struct {
	BlockTime time.Duration
}

var _ da.DataAvailabilityLayerClient = &DataAvailabilityLayerClient{}
var _ da.BlockRetriever = &DataAvailabilityLayerClient{}

// Init is called once to allow DA client to read configuration and initialize resources.
func (m *DataAvailabilityLayerClient) Init(_ [8]byte, config []byte, logger log.Logger) error {
	m.mut = &sync.Mutex{}
	m.logger = logger
	m.blocks = make(map[uint64][]*types.Block)
	m.daHeight = 1
	if len(config) > 0 {
		var err error
		m.config.BlockTime, err = time.ParseDuration(string(config))
		if err != nil {
			return err
		}
	} else {
		m.config.BlockTime = defaultBlockTime
	}
	return nil
}

// Start implements DataAvailabilityLayerClient interface.
func (m *DataAvailabilityLayerClient) Start() error {
	m.logger.Debug("Mock Data Availability Layer Client starting")
	go func() {
		for {
			time.Sleep(m.config.BlockTime)
			m.updateDAHeight()
		}
	}()
	return nil
}

// Stop implements DataAvailabilityLayerClient interface.
func (m *DataAvailabilityLayerClient) Stop() error {
	m.logger.Debug("Mock Data Availability Layer Client stopped")
	return nil
}

// SubmitBlock submits the passed in block to the DA layer.
// This should create a transaction which (potentially)
// triggers a state transition in the DA layer.
func (m *DataAvailabilityLayerClient) SubmitBlock(block *types.Block) da.ResultSubmitBlock {
	daHeight := atomic.LoadUint64(&m.daHeight)
	m.logger.Debug("Submitting block to DA layer!", "height", block.Header.Height, "dataLayerHeight", daHeight)

	m.mut.Lock()
	m.blocks[m.daHeight] = append(m.blocks[m.daHeight], block)
	m.mut.Unlock()

	return da.ResultSubmitBlock{
		BaseResult: da.BaseResult{
			Code:     da.StatusSuccess,
			Message:  "OK",
			DAHeight: daHeight,
		},
	}
}

// CheckBlockAvailability queries DA layer to check data availability of block corresponding to given header.
func (m *DataAvailabilityLayerClient) CheckBlockAvailability(daHeight uint64) da.ResultCheckBlock {
	blocksRes := m.RetrieveBlocks(daHeight)
	return da.ResultCheckBlock{BaseResult: da.BaseResult{Code: blocksRes.Code}, DataAvailable: len(blocksRes.Blocks) > 0}
}

// RetrieveBlocks returns block at given height from data availability layer.
func (m *DataAvailabilityLayerClient) RetrieveBlocks(daHeight uint64) da.ResultRetrieveBlocks {
	if daHeight >= atomic.LoadUint64(&m.daHeight) {
		return da.ResultRetrieveBlocks{BaseResult: da.BaseResult{Code: da.StatusError, Message: "block not found"}}
	}

	var blocks []*types.Block
	m.mut.Lock()
	blocks = m.blocks[daHeight]
	m.mut.Unlock()

	return da.ResultRetrieveBlocks{BaseResult: da.BaseResult{Code: da.StatusSuccess}, Blocks: blocks}
}

func (m *DataAvailabilityLayerClient) updateDAHeight() {
	blockStep := rand.Uint64()%10 + 1
	atomic.AddUint64(&m.daHeight, blockStep)
}
