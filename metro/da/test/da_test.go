package test

import (
	"encoding/json"
	"math/rand"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/libs/log"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	"github.com/tendermint/tendermint/metro/da"
	"github.com/tendermint/tendermint/metro/da/celestia"
	cmock "github.com/tendermint/tendermint/metro/da/celestia/mock"
	grpcda "github.com/tendermint/tendermint/metro/da/grpc"
	"github.com/tendermint/tendermint/metro/da/grpc/mockserv"
	"github.com/tendermint/tendermint/metro/da/mock"
	"github.com/tendermint/tendermint/metro/da/registry"
	"github.com/tendermint/tendermint/state"
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
	b1 := getRandomBlock(1, 10)
	b2 := getRandomBlock(2, 10)

	resp := dalc.SubmitBlock(b1)
	h1 := resp.DAHeight
	assert.Equal(da.StatusSuccess, resp.Code)

	resp = dalc.SubmitBlock(b2)
	h2 := resp.DAHeight
	assert.Equal(da.StatusSuccess, resp.Code)

	// wait a bit more than mockDaBlockTime, so rollmint blocks can be "included" in mock block
	time.Sleep(mockDaBlockTime + 20*time.Millisecond)

	check := dalc.CheckBlockAvailability(h1)
	assert.Equal(da.StatusSuccess, check.Code)
	assert.True(check.DataAvailable)

	check = dalc.CheckBlockAvailability(h2)
	assert.Equal(da.StatusSuccess, check.Code)
	assert.True(check.DataAvailable)

	// this height should not be used by DALC
	check = dalc.CheckBlockAvailability(h1 - 1)
	assert.Equal(da.StatusSuccess, check.Code)
	assert.False(check.DataAvailable)
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
	countAtHeight := make(map[uint64]int)
	blocks := make(map[*types.Block]uint64)

	for i := uint64(1); i < 100; i++ {
		b := getRandomBlock(i, rand.Int()%20)
		resp := dalc.SubmitBlock(b)
		require.Equal(da.StatusSuccess, resp.Code, resp.Message)
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
		containCheck := make(map[string]struct{})
		for _, rb := range ret.Blocks {
			hash := rb.Hash()
			containCheck[string(hash)] = struct{}{}
		}
		_, has := containCheck[string(b.Hash())]
		assert.True(has)
	}
}

// copy-pasted from store/store_test.go
func getRandomBlock(height uint64, nTxs int) *types.Block {
	txs := make([]types.Tx, nTxs)
	for i := 0; i < nTxs; i++ {
		txs[i] = getRandomTx()
	}

	return makeBlock(int64(height), txs, randValidatorSet(1))
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

func makeBlock(height int64, txs []types.Tx, valset *types.ValidatorSet) *types.Block {
	// Build base block with block data.
	block := types.MakeBlock(height, txs, &types.Commit{}, types.EvidenceData{}.Evidence)

	// Set time.
	timestamp := time.Now()

	// Fill rest of header with state data.
	block.Header.Populate(
		state.InitStateVersion.Consensus, "aaddffgg",
		timestamp, block.LastCommit.BlockID,
		valset.Hash(), valset.Hash(),
		types.HashConsensusParams(*types.DefaultConsensusParams()), valset.Hash(), valset.Hash(),
		valset.Proposer.Address,
	)
	return block
}

func randValidatorSet(numValidators int) *types.ValidatorSet {
	validators := make([]*types.Validator, numValidators)
	totalVotingPower := int64(0)
	for i := 0; i < numValidators; i++ {
		validators[i] = randValidator(totalVotingPower)
		totalVotingPower += validators[i].VotingPower
	}
	return types.NewValidatorSet(validators)
}

func randValidator(totalVotingPower int64) *types.Validator {
	// this modulo limits the ProposerPriority/VotingPower to stay in the
	// bounds of MaxTotalVotingPower minus the already existing voting power:
	val := types.NewValidator(randPubKey(), int64(tmrand.Uint64()%uint64(types.MaxTotalVotingPower-totalVotingPower)))
	val.ProposerPriority = tmrand.Int64() % (types.MaxTotalVotingPower - totalVotingPower)
	return val
}

func randPubKey() crypto.PubKey {
	pubKey := make(ed25519.PubKey, ed25519.PubKeySize)
	copy(pubKey, tmrand.Bytes(32))
	return ed25519.PubKey(tmrand.Bytes(32))
}
