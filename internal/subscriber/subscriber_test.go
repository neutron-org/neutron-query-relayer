package subscriber_test

import (
	"context"
	"github.com/neutron-org/neutron-query-relayer/internal/subscriber"
	neutrontypes "github.com/neutron-org/neutron/x/interchainqueries/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/stretchr/testify/assert"
	"testing"
)

func TestDoneShouldEndSubscribe(t *testing.T) {
	cfgLogger := zap.NewProductionConfig()
	logger, err := cfgLogger.Build()
	require.NoError(t, err)

	queriesTasksQueue := make(chan neutrontypes.RegisteredQuery, 100)
	cfg := subscriber.SubscriberConfig{
		RPCAddress:   "",
		RESTAddress:  "",
		Timeout:      0,
		ConnectionID: "",
		WatchedTypes: nil,
		Registry:     nil,
	}
	s, err := subscriber.NewSubscriber(&cfg, logger)
	assert.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		// should terminate Subscribe() function
		cancel()
	}()

	err = s.Subscribe(ctx, queriesTasksQueue)
	assert.Equal(t, err, nil)
}
