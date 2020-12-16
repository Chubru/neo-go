package server

import (
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/native"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/network"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

func getUnitTestChain(t *testing.T) (*core.Blockchain, config.Config, *zap.Logger) {
	net := netmode.UnitTestNet
	configPath := "../../../config"
	cfg, err := config.Load(configPath, net)
	require.NoError(t, err, "could not load config")

	memoryStore := storage.NewMemoryStore()
	logger := zaptest.NewLogger(t)
	chain, err := core.NewBlockchain(memoryStore, cfg.ProtocolConfiguration, logger)
	require.NoError(t, err, "could not create chain")

	go chain.Run()

	return chain, cfg, logger
}

func getTestBlocks(t *testing.T) []*block.Block {
	// File "./testdata/testblocks.acc" was generated by function core._
	// ("neo-go/pkg/core/helper_test.go").
	// To generate new "./testdata/testblocks.acc", follow the steps:
	// 		1. Rename the function
	// 		2. Add specific test-case into "neo-go/pkg/core/blockchain_test.go"
	// 		3. Run tests with `$ make test`
	f, err := os.Open("testdata/testblocks.acc")
	require.Nil(t, err)
	br := io.NewBinReaderFromIO(f)
	nBlocks := br.ReadU32LE()
	require.Nil(t, br.Err)
	blocks := make([]*block.Block, 0, int(nBlocks))
	for i := 0; i < int(nBlocks); i++ {
		_ = br.ReadU32LE()
		b := block.New(netmode.UnitTestNet, false)
		b.DecodeBinary(br)
		require.Nil(t, br.Err)
		blocks = append(blocks, b)
	}
	return blocks
}

func initClearServerWithInMemoryChain(t *testing.T) (*core.Blockchain, *Server, *httptest.Server) {
	chain, cfg, logger := getUnitTestChain(t)

	serverConfig := network.NewServerConfig(cfg)
	server, err := network.NewServer(serverConfig, chain, logger)
	require.NoError(t, err)
	rpcServer := New(chain, cfg.ApplicationConfiguration.RPC, server, logger)
	errCh := make(chan error, 2)
	rpcServer.Start(errCh)

	handler := http.HandlerFunc(rpcServer.handleHTTPRequest)
	srv := httptest.NewServer(handler)

	return chain, &rpcServer, srv
}

func initServerWithInMemoryChain(t *testing.T) (*core.Blockchain, *Server, *httptest.Server) {
	chain, rpcServer, srv := initClearServerWithInMemoryChain(t)

	for _, b := range getTestBlocks(t) {
		require.NoError(t, chain.AddBlock(b))
	}
	return chain, rpcServer, srv
}

type FeerStub struct{}

func (fs *FeerStub) FeePerByte() int64 {
	return 0
}

func (fs *FeerStub) BlockHeight() uint32 {
	return 0
}

func (fs *FeerStub) GetUtilityTokenBalance(acc util.Uint160) *big.Int {
	return big.NewInt(1000000 * native.GASFactor)
}

func (fs FeerStub) P2PSigExtensionsEnabled() bool {
	return false
}

func (fs FeerStub) GetBaseExecFee() int64 {
	return interop.DefaultBaseExecFee
}
