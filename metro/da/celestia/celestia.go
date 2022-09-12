package celestia

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gogo/protobuf/proto"

	cnc "github.com/celestiaorg/go-cnc"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/metro"
	"github.com/tendermint/tendermint/metro/da"
	metroproto "github.com/tendermint/tendermint/proto/tendermint/metro"
)

// DataAvailabilityLayerClient use celestia-node public API.
type DataAvailabilityLayerClient struct {
	client *cnc.Client

	namespaceID [8]byte
	config      Config
	logger      log.Logger
}

var _ da.DataAvailabilityLayerClient = &DataAvailabilityLayerClient{}
var _ da.BlockRetriever = &DataAvailabilityLayerClient{}

// Config stores Celestia DALC configuration parameters.
type Config struct {
	BaseURL  string        `json:"base_url"`
	Timeout  time.Duration `json:"timeout"`
	GasLimit uint64        `json:"gas_limit"`
}

// Init initializes DataAvailabilityLayerClient instance.
func (c *DataAvailabilityLayerClient) Init(namespaceID [8]byte, config []byte, logger log.Logger) error {
	c.namespaceID = namespaceID
	c.logger = logger

	if len(config) > 0 {
		return json.Unmarshal(config, &c.config)
	}

	return nil
}

// Start prepares DataAvailabilityLayerClient to work.
func (c *DataAvailabilityLayerClient) Start() error {
	c.logger.Info("starting Celestia Data Availability Layer Client", "baseURL", c.config.BaseURL)
	var err error
	c.client, err = cnc.NewClient(c.config.BaseURL, cnc.WithTimeout(c.config.Timeout))
	return err
}

// Stop stops DataAvailabilityLayerClient.
func (c *DataAvailabilityLayerClient) Stop() error {
	c.logger.Info("stopping Celestia Data Availability Layer Client")
	return nil
}

// SubmitBlock submits a block to DA layer.
func (c *DataAvailabilityLayerClient) SubmitMultiBlock(mblock *metro.MultiBlock) da.ResultSubmitBlock {
	pmblock, err := mblock.ToProto()
	if err != nil {
		return da.ResultSubmitBlock{
			BaseResult: da.BaseResult{
				Code:    da.StatusError,
				Message: err.Error(),
			},
		}
	}
	blob, err := proto.Marshal(pmblock)
	if err != nil {
		return da.ResultSubmitBlock{
			BaseResult: da.BaseResult{
				Code:    da.StatusError,
				Message: err.Error(),
			},
		}
	}

	txResponse, err := c.client.SubmitPFD(context.TODO(), c.namespaceID, blob, c.config.GasLimit)
	if err != nil {
		return da.ResultSubmitBlock{
			BaseResult: da.BaseResult{
				Code:    da.StatusError,
				Message: err.Error(),
			},
		}
	}

	if txResponse.Code != 0 {
		return da.ResultSubmitBlock{
			BaseResult: da.BaseResult{
				Code:    da.StatusError,
				Message: fmt.Sprintf("Codespace: '%s', Code: %d, Message: %s", txResponse.Codespace, txResponse.Code, txResponse.RawLog),
			},
		}
	}

	return da.ResultSubmitBlock{
		BaseResult: da.BaseResult{
			Code:     da.StatusSuccess,
			Message:  "tx hash: " + txResponse.TxHash,
			DAHeight: int64(txResponse.Height),
		},
	}
}

// CheckBlockAvailability queries DA layer to check data availability of block at given height.
func (c *DataAvailabilityLayerClient) CheckBlockAvailability(dataLayerHeight int64) da.ResultCheckBlock {
	shares, err := c.client.NamespacedShares(context.TODO(), c.namespaceID, uint64(dataLayerHeight))
	if err != nil {
		return da.ResultCheckBlock{
			BaseResult: da.BaseResult{
				Code:    da.StatusError,
				Message: err.Error(),
			},
		}
	}

	return da.ResultCheckBlock{
		BaseResult: da.BaseResult{
			Code:     da.StatusSuccess,
			DAHeight: dataLayerHeight,
		},
		DataAvailable: len(shares) > 0,
	}
}

// RetrieveBlocks gets a batch of blocks from DA layer.
func (c *DataAvailabilityLayerClient) RetrieveBlocks(dataLayerHeight int64) da.ResultRetrieveBlocks {
	data, err := c.client.NamespacedData(context.TODO(), c.namespaceID, uint64(dataLayerHeight))
	if err != nil {
		return da.ResultRetrieveBlocks{
			BaseResult: da.BaseResult{
				Code:    da.StatusError,
				Message: err.Error(),
			},
		}
	}

	blocks := make([]*metro.MultiBlock, len(data))
	for i, msg := range data {
		var pmblock metroproto.MultiBlock
		err = proto.Unmarshal(msg, &pmblock)
		if err != nil {
			c.logger.Error("failed to unmarshal block", "daHeight", dataLayerHeight, "position", i, "error", err)
			continue
		}

		mblock, err := metro.MultiBlockFromProto(&pmblock)
		if err != nil {
			return da.ResultRetrieveBlocks{
				BaseResult: da.BaseResult{
					Code:    da.StatusError,
					Message: err.Error(),
				},
			}
		}
		blocks[i] = mblock
	}

	return da.ResultRetrieveBlocks{
		BaseResult: da.BaseResult{
			Code:     da.StatusSuccess,
			DAHeight: dataLayerHeight,
		},
		Blocks: blocks,
	}
}
