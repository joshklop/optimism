package bootstrap

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/artifacts"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/retryproxy"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-service/testutils/anvil"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/params"
	"github.com/stretchr/testify/require"
)

func TestSuperchain(t *testing.T) {
	for _, network := range networks {
		for _, version := range versions {
			t.Run(network+"-"+version, func(t *testing.T) {
				envVar := strings.ToUpper(network) + "_RPC_URL"
				rpcURL := os.Getenv(envVar)
				require.NotEmpty(t, rpcURL, "must specify RPC url via %s env var", envVar)
				testSuperchain(t, rpcURL, version)
			})
		}
	}
}

func testSuperchain(t *testing.T, forkRPCURL string, version string) {
	t.Parallel()

	if forkRPCURL == "" {
		t.Skip("forkRPCURL not set")
	}

	lgr := testlog.Logger(t, slog.LevelDebug)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	retryProxy := retryproxy.New(lgr, forkRPCURL)
	require.NoError(t, retryProxy.Start())
	t.Cleanup(func() {
		require.NoError(t, retryProxy.Stop())
	})

	runner, err := anvil.New(
		retryProxy.Endpoint(),
		lgr,
	)
	require.NoError(t, err)

	require.NoError(t, runner.Start(ctx))
	t.Cleanup(func() {
		require.NoError(t, runner.Stop())
	})

	out, err := Superchain(ctx, SuperchainConfig{
		L1RPCUrl:         runner.RPCUrl(),
		PrivateKey:       "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80",
		ArtifactsLocator: artifacts.MustNewLocatorFromTag("op-contracts/" + version),
		Logger:           lgr,

		SuperchainProxyAdminOwner:  common.Address{'S'},
		ProtocolVersionsOwner:      common.Address{'P'},
		Guardian:                   common.Address{'G'},
		Paused:                     false,
		RequiredProtocolVersion:    params.ProtocolVersionV0{Major: 1}.Encode(),
		RecommendedProtocolVersion: params.ProtocolVersionV0{Major: 2}.Encode(),
	})
	require.NoError(t, err)

	client, err := ethclient.Dial(runner.RPCUrl())
	require.NoError(t, err)

	addresses := []common.Address{
		out.SuperchainConfigProxy,
		out.SuperchainConfigImpl,
		out.SuperchainProxyAdmin,
		out.ProtocolVersionsImpl,
		out.ProtocolVersionsProxy,
	}
	for _, addr := range addresses {
		require.NotEmpty(t, addr)

		code, err := client.CodeAt(ctx, addr, nil)
		require.NoError(t, err)
		require.NotEmpty(t, code)
	}
}