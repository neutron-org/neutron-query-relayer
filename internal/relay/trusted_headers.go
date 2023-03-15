package relay

import (
	"context"

	"github.com/cosmos/ibc-go/v4/modules/core/exported"
)

// TrustedHeaderFetcher able to get trusted headers for a given height
type TrustedHeaderFetcher interface {
	// Fetch returns only one trusted Header for specified height
	Fetch(ctx context.Context, height uint64) (exported.Header, error)
}
