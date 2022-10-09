package mockserv

import (
	"context"
	"os"

	tmlog "github.com/tendermint/tendermint/libs/log"
	"google.golang.org/grpc"

	grpcda "github.com/tendermint/tendermint/metro/da/grpc"
	"github.com/tendermint/tendermint/metro/da/mock"
	"github.com/tendermint/tendermint/proto/tendermint/dalc"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

// GetServer creates and returns gRPC server instance.
func GetServer(conf grpcda.Config, mockConfig []byte) *grpc.Server {
	logger := tmlog.NewTMLogger(os.Stdout)

	srv := grpc.NewServer()
	mockImpl := &mockImpl{}
	err := mockImpl.mock.Init([8]byte{}, mockConfig, logger)
	if err != nil {
		logger.Error("failed to initialize mock DALC", "error", err)
		panic(err)
	}
	err = mockImpl.mock.Start()
	if err != nil {
		logger.Error("failed to start mock DALC", "error", err)
		panic(err)
	}
	dalc.RegisterDALCServiceServer(srv, mockImpl)
	return srv
}

type mockImpl struct {
	mock mock.DataAvailabilityLayerClient
}

func (m *mockImpl) SubmitBlock(_ context.Context, request *dalc.SubmitBlockRequest) (*dalc.SubmitBlockResponse, error) {
	b, err := types.BlockFromProto(request.Block)
	if err != nil {
		return nil, err
	}
	resp := m.mock.SubmitBlock(b)
	return &dalc.SubmitBlockResponse{
		Result: &dalc.DAResponse{
			Code:     dalc.StatusCode(resp.Code),
			Message:  resp.Message,
			DAHeight: resp.DAHeight,
		},
	}, nil
}

func (m *mockImpl) CheckBlockAvailability(_ context.Context, request *dalc.CheckBlockAvailabilityRequest) (*dalc.CheckBlockAvailabilityResponse, error) {
	resp := m.mock.CheckBlockAvailability(request.DAHeight)
	return &dalc.CheckBlockAvailabilityResponse{
		Result: &dalc.DAResponse{
			Code:    dalc.StatusCode(resp.Code),
			Message: resp.Message,
		},
		DataAvailable: resp.DataAvailable,
	}, nil
}

func (m *mockImpl) RetrieveBlocks(context context.Context, request *dalc.RetrieveBlocksRequest) (*dalc.RetrieveBlocksResponse, error) {
	resp := m.mock.RetrieveBlocks(request.DAHeight)
	blocks := make([]*tmproto.Block, len(resp.Blocks))
	for i, b := range resp.Blocks {
		tB, err := b.ToProto()
		if err != nil {
			return nil, err
		}
		blocks[i] = tB
	}
	return &dalc.RetrieveBlocksResponse{
		Result: &dalc.DAResponse{
			Code:    dalc.StatusCode(resp.Code),
			Message: resp.Message,
		},
		Blocks: blocks,
	}, nil
}
