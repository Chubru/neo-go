package rpcsrv

import (
	"fmt"
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
	"github.com/nspcc-dev/neo-go/pkg/services/oracle"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

const (
	notaryPath = "../notary/testdata/notary1.json"
	notaryPass = "one"
)

func getUnitTestChain(t testing.TB, enableOracle bool, enableNotary bool, disableIteratorSessions bool) (*core.Blockchain, OracleHandler, config.Config, *zap.Logger) {
	return getUnitTestChainWithCustomConfig(t, enableOracle, enableNotary, func(cfg *config.Config) {
		if disableIteratorSessions {
			cfg.ApplicationConfiguration.RPC.SessionEnabled = false
		}
		if enableNotary {
			cfg.ProtocolConfiguration.P2PSigExtensions = true
			cfg.ProtocolConfiguration.P2PNotaryRequestPayloadPoolSize = 1000
			cfg.ApplicationConfiguration.P2PNotary = config.P2PNotary{
				Enabled: true,
				UnlockWallet: config.Wallet{
					Path:     notaryPath,
					Password: notaryPass,
				},
			}
		} else {
			cfg.ApplicationConfiguration.P2PNotary.Enabled = false
		}
		if enableOracle {
			cfg.ApplicationConfiguration.Oracle.Enabled = true
			cfg.ApplicationConfiguration.Oracle.UnlockWallet = config.Wallet{
				Path:     "../oracle/testdata/oracle1.json",
				Password: "one",
			}
		}
	})
}
func getUnitTestChainWithCustomConfig(t testing.TB, enableOracle bool, enableNotary bool, customCfg func(configuration *config.Config)) (*core.Blockchain, OracleHandler, config.Config, *zap.Logger) {
	net := netmode.UnitTestNet
	configPath := "../../../config"
	cfg, err := config.Load(configPath, net)
	require.NoError(t, err, "could not load config")
	if customCfg != nil {
		customCfg(&cfg)
	}

	memoryStore := storage.NewMemoryStore()
	logger := zaptest.NewLogger(t)
	chain, err := core.NewBlockchain(memoryStore, cfg.Blockchain(), logger)
	require.NoError(t, err, "could not create chain")

	var orc OracleHandler
	if enableOracle {
		orc, err = oracle.NewOracle(oracle.Config{
			Log:     logger,
			Network: netmode.UnitTestNet,
			MainCfg: cfg.ApplicationConfiguration.Oracle,
			Chain:   chain,
		})
		require.NoError(t, err)
		chain.SetOracle(orc.(*oracle.Oracle))
	}

	go chain.Run()
	t.Cleanup(chain.Close)

	return chain, orc, cfg, logger
}

func getTestBlocks(t *testing.T) []*block.Block {
	// File "./testdata/testblocks.acc" was generated by function core.TestCreateBasicChain
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
		b := &block.Block{}
		b.DecodeBinary(br)
		require.Nil(t, br.Err)
		blocks = append(blocks, b)
	}
	return blocks
}

func initClearServerWithServices(t testing.TB, needOracle bool, needNotary bool, disableIteratorsSessions bool) (*core.Blockchain, *Server, *httptest.Server) {
	chain, orc, cfg, logger := getUnitTestChain(t, needOracle, needNotary, disableIteratorsSessions)
	return wrapUnitTestChain(t, chain, orc, cfg, logger)
}

func wrapUnitTestChain(t testing.TB, chain *core.Blockchain, orc OracleHandler, cfg config.Config, logger *zap.Logger) (*core.Blockchain, *Server, *httptest.Server) {
	serverConfig, err := network.NewServerConfig(cfg)
	require.NoError(t, err)
	serverConfig.UserAgent = fmt.Sprintf(config.UserAgentFormat, "0.98.6-test")
	serverConfig.Addresses = []config.AnnounceableAddress{{Address: ":0"}}
	server, err := network.NewServer(serverConfig, chain, chain.GetStateSyncModule(), logger)
	require.NoError(t, err)
	errCh := make(chan error, 2)
	rpcServer := New(chain, cfg.ApplicationConfiguration.RPC, server, orc, logger, errCh)
	rpcServer.Start()
	t.Cleanup(rpcServer.Shutdown)

	handler := http.HandlerFunc(rpcServer.handleHTTPRequest)
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return chain, &rpcServer, srv
}

func initClearServerWithCustomConfig(t testing.TB, ccfg func(configuration *config.Config)) (*core.Blockchain, *Server, *httptest.Server) {
	chain, orc, cfg, logger := getUnitTestChainWithCustomConfig(t, false, false, ccfg)
	return wrapUnitTestChain(t, chain, orc, cfg, logger)
}

func initClearServerWithInMemoryChain(t testing.TB) (*core.Blockchain, *Server, *httptest.Server) {
	return initClearServerWithServices(t, false, true, false)
}

func initServerWithInMemoryChain(t *testing.T) (*core.Blockchain, *Server, *httptest.Server) {
	chain, rpcServer, srv := initClearServerWithInMemoryChain(t)

	for _, b := range getTestBlocks(t) {
		require.NoError(t, chain.AddBlock(b))
	}
	return chain, rpcServer, srv
}

func initServerWithInMemoryChainAndServices(t *testing.T, needOracle bool, needNotary bool, disableIteratorSessions bool) (*core.Blockchain, *Server, *httptest.Server) {
	chain, rpcServer, srv := initClearServerWithServices(t, needOracle, needNotary, disableIteratorSessions)

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

func (fs FeerStub) GetBaseExecFee() int64 {
	return interop.DefaultBaseExecFee
}
