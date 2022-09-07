package relay

import (
	"context"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
)

// TrustedHeaderFetcher able to get trusted headers for a given height
type TrustedHeaderFetcher interface {
	// Fetch returns two Headers for height and height+1 packed into *codectypes.Any value for
	Fetch(ctx context.Context, height uint64) (header *codectypes.Any, nextHeader *codectypes.Any, err error)
}
