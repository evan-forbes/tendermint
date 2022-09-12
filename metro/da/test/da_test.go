package test

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/metro"
	"github.com/tendermint/tendermint/metro/da"
	"github.com/tendermint/tendermint/metro/da/celestia"
	cmock "github.com/tendermint/tendermint/metro/da/celestia/mock"
	grpcda "github.com/tendermint/tendermint/metro/da/grpc"
	"github.com/tendermint/tendermint/metro/da/grpc/mockserv"
	"github.com/tendermint/tendermint/metro/da/mock"
	"github.com/tendermint/tendermint/metro/da/registry"
	"github.com/tendermint/tendermint/proto/tendermint/version"
	"github.com/tendermint/tendermint/types"
)

const mockDaBlockTime = 100 * time.Millisecond

var testNamespaceID = [8]byte{0, 1, 2, 3, 4, 5, 6, 7}

func TestLifecycle(t *testing.T) {
	srv := startMockGRPCServ(t)
	defer srv.GracefulStop()

	for _, dalc := range registry.RegisteredClients() {
		t.Run(dalc, func(t *testing.T) {
			doTestLifecycle(t, registry.GetClient(dalc))
		})
	}
}

func doTestLifecycle(t *testing.T, dalc da.DataAvailabilityLayerClient) {
	require := require.New(t)

	err := dalc.Init(testNamespaceID, []byte{}, log.NewNopLogger())
	require.NoError(err)

	err = dalc.Start()
	require.NoError(err)

	err = dalc.Stop()
	require.NoError(err)
}

func TestDALC(t *testing.T) {
	grpcServer := startMockGRPCServ(t)
	defer grpcServer.GracefulStop()

	httpServer := startMockCelestiaNodeServer(t)
	defer httpServer.Stop()

	for _, dalc := range registry.RegisteredClients() {
		t.Run(dalc, func(t *testing.T) {
			fmt.Println("dalc type", dalc)
			doTestDALC(t, registry.GetClient(dalc))
		})
	}
}

func doTestDALC(t *testing.T, dalc da.DataAvailabilityLayerClient) {
	require := require.New(t)
	assert := assert.New(t)

	// mock DALC will advance block height every 100ms
	conf := []byte{}
	if _, ok := dalc.(*mock.DataAvailabilityLayerClient); ok {
		conf = []byte(mockDaBlockTime.String())
	}
	if _, ok := dalc.(*celestia.DataAvailabilityLayerClient); ok {
		config := celestia.Config{
			BaseURL:  "http://localhost:26658",
			Timeout:  30 * time.Second,
			GasLimit: 3000000,
		}
		conf, _ = json.Marshal(config)
	}
	err := dalc.Init(testNamespaceID, conf, log.NewNopLogger())
	require.NoError(err)

	err = dalc.Start()
	require.NoError(err)

	// wait a bit more than mockDaBlockTime, so mock can "produce" some blocks
	time.Sleep(mockDaBlockTime + 20*time.Millisecond)

	// only blocks b1 and b2 will be submitted to DA
	b1 := getRandomMultiBlock(1, 10, 10)
	b2 := getRandomMultiBlock(11, 20, 10)

	resp := dalc.SubmitMultiBlock(b1)
	h1 := resp.DAHeight
	assert.Equal(da.StatusSuccess, resp.Code, 1)
	fmt.Println("message from submit 1:", resp.Message, len(b1.Blocks))

	resp = dalc.SubmitMultiBlock(b2)
	h2 := resp.DAHeight
	assert.Equal(da.StatusSuccess, resp.Code, 2)
	fmt.Println("message from submit", resp.Message, len(b2.Blocks))

	// wait a bit more than mockDaBlockTime, so optimint blocks can be "included" in mock block
	time.Sleep(mockDaBlockTime + 20*time.Millisecond)

	check := dalc.CheckBlockAvailability(h1)
	assert.Equal(da.StatusSuccess, check.Code, 3)
	assert.True(check.DataAvailable)

	check = dalc.CheckBlockAvailability(h2)
	assert.Equal(da.StatusSuccess, check.Code, 4)
	assert.True(check.DataAvailable)

	// this height should not be used by DALC
	check = dalc.CheckBlockAvailability(h1 - 1)
	assert.Equal(da.StatusSuccess, check.Code, 5)
	assert.True(check.DataAvailable)
}

func TestRetrieve(t *testing.T) {
	grpcServer := startMockGRPCServ(t)
	defer grpcServer.GracefulStop()

	httpServer := startMockCelestiaNodeServer(t)
	defer httpServer.Stop()

	for _, client := range registry.RegisteredClients() {
		t.Run(client, func(t *testing.T) {
			dalc := registry.GetClient(client)
			_, ok := dalc.(da.BlockRetriever)
			if ok {
				doTestRetrieve(t, dalc)
			}
		})
	}
}

func startMockGRPCServ(t *testing.T) *grpc.Server {
	t.Helper()
	conf := grpcda.DefaultConfig
	srv := mockserv.GetServer(conf, []byte(mockDaBlockTime.String()))
	lis, err := net.Listen("tcp", conf.Host+":"+strconv.Itoa(conf.Port))
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		_ = srv.Serve(lis)
	}()
	return srv
}

func startMockCelestiaNodeServer(t *testing.T) *cmock.Server {
	t.Helper()
	httpSrv := cmock.NewServer(mockDaBlockTime, log.NewNopLogger())
	l, err := net.Listen("tcp4", ":26658")
	if err != nil {
		t.Fatal("failed to create listener for mock celestia-node RPC server", "error", err)
	}
	err = httpSrv.Start(l)
	if err != nil {
		t.Fatal("can't start mock celestia-node RPC server")
	}
	return httpSrv
}

func doTestRetrieve(t *testing.T, dalc da.DataAvailabilityLayerClient) {
	require := require.New(t)
	assert := assert.New(t)

	// mock DALC will advance block height every 100ms
	conf := []byte{}
	if _, ok := dalc.(*mock.DataAvailabilityLayerClient); ok {
		conf = []byte(mockDaBlockTime.String())
	}
	if _, ok := dalc.(*celestia.DataAvailabilityLayerClient); ok {
		config := celestia.Config{
			BaseURL:  "http://localhost:26658",
			Timeout:  30 * time.Second,
			GasLimit: 3000000,
		}
		conf, _ = json.Marshal(config)
	}
	err := dalc.Init(testNamespaceID, conf, log.NewNopLogger())
	require.NoError(err)

	err = dalc.Start()
	require.NoError(err)

	// wait a bit more than mockDaBlockTime, so mock can "produce" some blocks
	time.Sleep(mockDaBlockTime + 20*time.Millisecond)

	retriever := dalc.(da.BlockRetriever)
	countAtHeight := make(map[int64]int)
	blocks := make(map[*metro.MultiBlock]int64)

	for i := uint64(0); i < 100; i++ {
		b := getRandomMultiBlock(0, 100, rand.Int()%20)

		resp := dalc.SubmitMultiBlock(b)
		assert.Equal(da.StatusSuccess, resp.Code, resp.Message)
		time.Sleep(time.Duration(rand.Int63() % mockDaBlockTime.Milliseconds()))

		countAtHeight[resp.DAHeight]++
		blocks[b] = resp.DAHeight
	}

	// wait a bit more than mockDaBlockTime, so mock can "produce" last blocks
	time.Sleep(mockDaBlockTime + 20*time.Millisecond)

	for h, cnt := range countAtHeight {
		t.Log("Retrieving block, DA Height", h)
		ret := retriever.RetrieveBlocks(h)
		assert.Equal(da.StatusSuccess, ret.Code, ret.Message)
		require.NotEmpty(ret.Blocks, h)
		assert.Len(ret.Blocks, cnt, h)
	}

	for b, h := range blocks {
		ret := retriever.RetrieveBlocks(h)
		assert.Equal(da.StatusSuccess, ret.Code, h)
		require.NotEmpty(ret.Blocks, h)
		assert.Contains(ret.Blocks, b, h)
	}
}

// copy-pasted from store/store_test.go
func getRandomMultiBlock(startHeight, endHeight int64, nTxs int) *metro.MultiBlock {
	mb := metro.MultiBlock{}
	propAddr := crypto.AddressHash([]byte("validator_address"))
	for i := startHeight; i < endHeight; i++ {
		bdata := types.Data{
			Txs: make(types.Txs, nTxs),
		}
		for i := 0; i < nTxs; i++ {
			bdata.Txs[i] = getRandomTx()
		}
		bdataHash := bdata.Hash()
		block := &metro.SimpleBlock{
			Header: types.Header{
				Height:          i,
				Version:         version.Consensus{Block: 11},
				ProposerAddress: propAddr,
				DataHash:        bdataHash,
			},
			Data: bdata,
		}
		copy(block.Header.AppHash[:], getRandomBytes(32))
		block.EvidenceHash = block.Evidence.Hash()

		// TODO(tzdybal): see https://github.com/tendermint/tendermint/issues/143
		if nTxs == 0 {
			block.Data.Txs = nil
		}

		mb.Blocks = append(mb.Blocks, block)
	}

	mb.LastCommit = &types.Commit{}

	return &mb
}

func getRandomTx() types.Tx {
	size := rand.Int()%100 + 100
	return types.Tx(getRandomBytes(size))
}

func getRandomBytes(n int) []byte {
	data := make([]byte, n)
	_, _ = rand.Read(data)
	return data
}
