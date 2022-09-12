package mockserv

import (
	"context"
	"fmt"
	"os"

	tmlog "github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/metro"
	"google.golang.org/grpc"

	grpcda "github.com/tendermint/tendermint/metro/da/grpc"
	"github.com/tendermint/tendermint/metro/da/mock"
	"github.com/tendermint/tendermint/proto/tendermint/dalc"
	metroproto "github.com/tendermint/tendermint/proto/tendermint/metro"
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
	fmt.Println("---------------------------------- mock multi len", len(request.Block.Blocks))
	mblock, err := metro.MultiBlockFromProto(request.Block)
	if err != nil {
		fmt.Println("errrrrrrrrrrrrrrrrrr", err)
		return nil, err
	}
	resp := m.mock.SubmitMultiBlock(mblock)
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
	blocks := make([]*metroproto.MultiBlock, len(resp.Blocks))
	for i := range resp.Blocks {
		pmblock, err := resp.Blocks[i].ToProto()
		if err != nil {
			return nil, err
		}
		blocks[i] = pmblock
	}
	return &dalc.RetrieveBlocksResponse{
		Result: &dalc.DAResponse{
			Code:    dalc.StatusCode(resp.Code),
			Message: resp.Message,
		},
		Blocks: blocks,
	}, nil
}
