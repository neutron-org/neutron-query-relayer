package relay

import "context"

// TxSubmitChecker runs in background and updates submitted tx statuses
type TxSubmitChecker interface {
	Run(ctx context.Context) error
}
