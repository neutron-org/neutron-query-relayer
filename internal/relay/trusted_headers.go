package relay

import (
	"context"
	tmclient "github.com/cosmos/ibc-go/v7/modules/light-clients/07-tendermint"
)

// TrustedHeaderFetcher able to get trusted headers for a given height
type TrustedHeaderFetcher interface {
	// Fetch returns only one trusted Header for specified height
	Fetch(ctx context.Context, height uint64) (*tmclient.Header, error)
}
