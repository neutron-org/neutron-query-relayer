package relay

import (
	"fmt"
	"time"
)

type Resubmitter struct {
	retryDelay time.Duration
	pending    map[uint64]uint8
	limit      uint8
}

func NewResubmitter(retryDelay time.Duration, limit uint8) *Resubmitter {
	return &Resubmitter{
		retryDelay: retryDelay,
		limit:      limit,
		pending:    make(map[uint64]uint8),
	}
}

func (r *Resubmitter) Add(queryID uint64, cb func()) error {
	p, ok := r.pending[queryID]
	if !ok {
		r.pending[queryID] = 1
	} else {
		r.pending[queryID] = p + 1
	}
	if r.pending[queryID] > r.limit {
		return fmt.Errorf("queryID %d has been resubmitted %d times, which is more than the limit %d", queryID, r.pending)
	}

	go func() {
		var t = time.NewTimer(r.retryDelay)
		select {
		case <-t.C:
			cb()
		}
	}()

	return nil
}

func (r *Resubmitter) IsPending(queryID uint64) bool {
	_, ok := r.pending[queryID]
	return ok
}

func (r *Resubmitter) Remove(queryID uint64) {
	delete(r.pending, queryID)
}
