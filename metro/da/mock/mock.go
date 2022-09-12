package mock

import (
	"encoding/binary"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/metro"
	"github.com/tendermint/tendermint/metro/da"
)

// DataAvailabilityLayerClient is intended only for usage in tests.
// It does actually ensures DA - it stores data in-memory.
type DataAvailabilityLayerClient struct {
	logger   log.Logger
	daHeight int64
	config   config

	mtx     *sync.Mutex
	mblocks map[int64][]*metro.MultiBlock
}

const defaultBlockTime = 3 * time.Second

type config struct {
	BlockTime time.Duration
}

var _ da.DataAvailabilityLayerClient = &DataAvailabilityLayerClient{}
var _ da.BlockRetriever = &DataAvailabilityLayerClient{}

// Init is called once to allow DA client to read configuration and initialize resources.
func (m *DataAvailabilityLayerClient) Init(_ [8]byte, config []byte, logger log.Logger) error {
	m.logger = logger
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
	m.mtx = &sync.Mutex{}
	m.mblocks = make(map[int64][]*metro.MultiBlock)
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
func (m *DataAvailabilityLayerClient) SubmitMultiBlock(mblock *metro.MultiBlock) da.ResultSubmitBlock {
	daHeight := atomic.LoadInt64(&m.daHeight)
	if len(mblock.Blocks) == 0 {
		return da.ResultSubmitBlock{BaseResult: da.BaseResult{Code: da.StatusError, Message: "multi block must have at least one block"}}
	}

	lastBlock := mblock.Blocks[len(mblock.Blocks)-1]
	m.logger.Debug("Submitting block to DA layer!", "height", lastBlock.Header.Height, "dataLayerHeight", daHeight)

	m.mtx.Lock()
	m.mblocks[daHeight] = append(m.mblocks[daHeight], mblock)
	m.mtx.Unlock()

	// pmblock, err := mblock.ToProto()
	// if err != nil {
	// 	return da.ResultSubmitBlock{BaseResult: da.BaseResult{Code: da.StatusError, Message: err.Error()}}
	// }

	// blob, err := proto.Marshal(pmblock)
	// if err != nil {
	// 	return da.ResultSubmitBlock{BaseResult: da.BaseResult{Code: da.StatusError, Message: err.Error()}}
	// }

	return da.ResultSubmitBlock{
		BaseResult: da.BaseResult{
			Code:     da.StatusSuccess,
			Message:  "OK",
			DAHeight: daHeight,
		},
	}
}

// CheckBlockAvailability queries DA layer to check data availability of block corresponding to given header.
func (m *DataAvailabilityLayerClient) CheckBlockAvailability(daHeight int64) da.ResultCheckBlock {
	return da.ResultCheckBlock{BaseResult: da.BaseResult{Code: da.StatusSuccess}, DataAvailable: true}
}

// RetrieveBlocks returns block at given height from data availability layer.
func (m *DataAvailabilityLayerClient) RetrieveBlocks(daHeight int64) da.ResultRetrieveBlocks {
	if daHeight >= atomic.LoadInt64(&m.daHeight) {
		return da.ResultRetrieveBlocks{BaseResult: da.BaseResult{Code: da.StatusError, Message: "block not found"}}
	}

	m.mtx.Lock()
	mblocks, has := m.mblocks[daHeight]
	m.mtx.Unlock()

	if !has {
		return da.ResultRetrieveBlocks{BaseResult: da.BaseResult{Code: da.StatusError, Message: "no blocks found at height"}}
	}

	return da.ResultRetrieveBlocks{BaseResult: da.BaseResult{Code: da.StatusSuccess}, Blocks: mblocks}
}

func getPrefix(daHeight uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, daHeight)
	return b
}

func getKey(daHeight uint64, height uint64) []byte {
	b := make([]byte, 16)
	binary.BigEndian.PutUint64(b, daHeight)
	binary.BigEndian.PutUint64(b[8:], height)
	return b
}

func (m *DataAvailabilityLayerClient) updateDAHeight() {
	blockStep := rand.Uint64()%10 + 1
	atomic.AddInt64(&m.daHeight, int64(blockStep))
}
