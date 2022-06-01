package relay

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/cosmos/relayer/v2/relayer/provider/cosmos"

	"go.uber.org/zap"

	"github.com/cosmos/relayer/v2/cmd"
	"github.com/cosmos/relayer/v2/relayer"
)

// GetChainFromFile reads a JSON-formatted chain from the named file and adds it to a's chains.
func GetChainFromFile(logger *zap.Logger, homepath, file string, debug bool) (*relayer.Chain, error) {
	// If the user passes in a file, attempt to read the chain config from that file
	var pcw cmd.ProviderConfigWrapper
	if _, err := os.Stat(file); err != nil {
		return nil, fmt.Errorf("failed to get FileInfo for ")
	}

	byt, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read provider config file: %w", err)
	}

	if err = json.Unmarshal(byt, &pcw); err != nil {
		return nil, fmt.Errorf("failed to unmarshal provider config file: %w", err)
	}

	prov, err := pcw.Value.NewProvider(
		logger,
		homepath,
		debug,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to build ChainProvider for %s: %w", file, err)
	}

	provConcrete, ok := prov.(*cosmos.CosmosProvider)
	if !ok {
		return nil, fmt.Errorf("failed to patch CosmosProvider config (type cast failed)")
	}
	provConcrete.Config.KeyDirectory = homepath

	return relayer.NewChain(logger, prov, debug), nil
}
