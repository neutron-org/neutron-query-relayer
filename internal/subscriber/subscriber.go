package subscriber

import (
	"context"
	"fmt"
	restClient "github.com/neutron-org/neutron-query-relayer/internal/subscriber/querier/client"
	"strconv"
	"time"

	"github.com/tendermint/tendermint/rpc/client/http"
	tmtypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/types"
	"go.uber.org/zap"

	"github.com/neutron-org/neutron-query-relayer/internal/registry"
	neutrontypes "github.com/neutron-org/neutron/x/interchainqueries/types"
	"github.com/zyedidia/generic/queue"
)

var (
	unsubscribeTimeout = time.Second * 5
)

// NewSubscriber creates a new Subscriber instance ready to subscribe on the given chain's events.
func NewSubscriber(
	rpcAddress string,
	targetChainID string,
	targetConnectionID string,
	registry *registry.Registry,
	watchedTypes []neutrontypes.InterchainQueryType,
	logger *zap.Logger,
) (*Subscriber, error) {
	rpcClient, err := http.New(rpcAddress, "/websocket")
	if err != nil {
		return nil, fmt.Errorf("could not create new tendermint rpcClient: %w", err)
	}

	if err = rpcClient.Start(); err != nil {
		return nil, fmt.Errorf("could not start tendermint rpcClient: %w", err)
	}

	watchedTypesMap := make(map[neutrontypes.InterchainQueryType]struct{})
	for _, queryType := range watchedTypes {
		watchedTypesMap[queryType] = struct{}{}
	}

	return &Subscriber{
		rpcClient: rpcClient,
		restClient: restClient.NewHTTPClientWithConfig(nil, &restClient.TransportConfig{
			Host:     rpcAddress,
			BasePath: "/",
			Schemes:  []string{"https"},
		}),

		rpcAddress:         rpcAddress,
		targetChainID:      targetChainID,
		targetConnectionID: targetConnectionID,
		registry:           registry,
		logger:             logger,
		watchedTypes:       watchedTypesMap,

		activeQueries: map[string]*neutrontypes.RegisteredQuery{},
	}, nil
}

// Subscriber is responsible for subscribing on chain's ICQ events. It parses incoming events,
// filters them in accordance with the Registry configuration and watchedTypes, and provides a
// stream of split to KV and TX messages.
type Subscriber struct {
	rpcClient  *http.HTTP                 // Used to subscribe to events
	restClient *restClient.HTTPAPIConsole // Used to run Neutron-specific queries using the REST

	rpcAddress         string
	targetChainID      string
	targetConnectionID string
	registry           *registry.Registry
	logger             *zap.Logger
	watchedTypes       map[neutrontypes.InterchainQueryType]struct{}

	activeQueries map[string]*neutrontypes.RegisteredQuery
}

// Subscribe TODO(oopcode).
func (s *Subscriber) Subscribe(ctx context.Context, tasks *queue.Queue[neutrontypes.RegisteredQuery]) error {
	queries, err := s.getNeutronRegisteredQueries()
	if err != nil {
		return fmt.Errorf("could not getNeutronRegisteredQueries: %w", err)
	}
	s.activeQueries = queries

	updateEvents, err := s.rpcClient.Subscribe(ctx, s.subscriberName(), s.subscribeQueryUpdated())
	if err != nil {
		return fmt.Errorf("could not subscribe to events: %w", err)
	}

	removeEvents, err := s.rpcClient.Subscribe(ctx, s.subscriberName(), s.subscribeQueryRemoved())
	if err != nil {
		return fmt.Errorf("could not subscribe to events: %w", err)
	}

	blockEvents, err := s.rpcClient.Subscribe(ctx, s.subscriberName(), s.subscribeQueryBlock())
	if err != nil {
		return fmt.Errorf("could not subscribe to events: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			s.unsubscribe()
			return nil
		case event := <-blockEvents:
			if err := s.processBlockEvent(event, tasks); err != nil {
				return fmt.Errorf("failed to processblockEvent: %w", err)
			}
		case event := <-updateEvents:
			if err := s.processUpdateEvent(event); err != nil {
				return fmt.Errorf("failed to processUpdateEvent: %w", err)
			}
		case event := <-removeEvents:
			if err := s.processRemoveEvent(event); err != nil {
				return fmt.Errorf("failed to processRemoveEvent: %w", err)
			}
		}
	}
}

func (s *Subscriber) unsubscribe() {
	ctx, cancel := context.WithTimeout(context.Background(), unsubscribeTimeout)
	defer cancel()

	if err := s.rpcClient.Unsubscribe(ctx, s.subscriberName(), s.subscribeQueryUpdated()); err != nil {
		s.logger.Error("failed to Unsubscribe from tm events",
			zap.Error(err), zap.String("ActiveQuery", s.subscribeQueryUpdated()))
	}

	if err := s.rpcClient.Unsubscribe(ctx, s.subscriberName(), s.subscribeQueryRemoved()); err != nil {
		s.logger.Error("failed to Unsubscribe from tm events",
			zap.Error(err), zap.String("ActiveQuery", s.subscribeQueryRemoved()))
	}

	if err := s.rpcClient.Unsubscribe(ctx, s.subscriberName(), s.subscribeQueryBlock()); err != nil {
		s.logger.Error("failed to Unsubscribe from tm events",
			zap.Error(err), zap.String("ActiveQuery", s.subscribeQueryBlock()))
	}
}

func (s *Subscriber) processBlockEvent(event tmtypes.ResultEvent, tasks *queue.Queue[neutrontypes.RegisteredQuery]) error {
	// TODO(oopcode): watchedTypes

	currentHeight, err := s.extractBlockHeight(event)
	if err != nil {
		return fmt.Errorf("failed to extractBlockHeight: %w", err)
	}

	for _, activeQuery := range s.activeQueries {
		// Skip the ActiveQuery if we didn't reach the update time.
		if currentHeight < (activeQuery.LastSubmittedResultLocalHeight + activeQuery.UpdatePeriod) {
			continue
		}

		// Send the query to the task queue.
		tasks.Enqueue(*activeQuery)

		// Set the LastSubmittedResultLocalHeight to the current height.
		activeQuery.LastSubmittedResultLocalHeight = currentHeight
	}

	return nil
}

// processUpdateEvent TODO(oopcode).
func (s *Subscriber) processUpdateEvent(event tmtypes.ResultEvent) error {
	ok, err := s.checkEvents(event)
	if err != nil {
		return fmt.Errorf("failed to checkEvents: %w", err)
	}
	if !ok {
		return nil
	}

	// There can be multiple events of the same type associated with our connection id in a
	// single tmtypes.ResultEvent value. We need to process all of them.
	var events = event.Events
	for idx := range events[connectionIdAttr] {
		if !s.isWatchedAddress(events[ownerAttr][idx]) {
			continue
		}

		// Load all information about the query directly from ToNeutronRegisteredQuery.
		var queryID = events[queryIdAttr][idx]
		query, err := s.getNeutronRegisteredQuery(queryID)
		if err != nil {
			return fmt.Errorf("failed to getNeutronRegisteredQuery: %w", err)
		}

		// Save the updated query information to state.
		s.activeQueries[queryID] = query
	}

	return nil
}

// processRemoveEvent TODO(oopcode).
func (s *Subscriber) processRemoveEvent(event tmtypes.ResultEvent) error {
	ok, err := s.checkEvents(event)
	if err != nil {
		return fmt.Errorf("failed to checkEvents: %w", err)
	}
	if !ok {
		return nil
	}

	// There can be multiple events of the same type associated with our connection id in a
	// single tmtypes.ResultEvent value. We need to process all of them.
	var events = event.Events
	for idx := range events[connectionIdAttr] {
		if !s.isWatchedAddress(events[ownerAttr][idx]) {
			continue
		}

		// Delete the query from the active queries list.
		var queryID = events[queryIdAttr][idx]
		delete(s.activeQueries, queryID)
	}

	return nil
}

func (s *Subscriber) parseQueryID(events map[string][]string, idx int) (uint64, error) {
	queryIdStr := events[queryIdAttr][idx]
	queryId, err := strconv.ParseUint(queryIdStr, 10, 64)
	if err != nil {

		return 0, fmt.Errorf("failed to ParseUint: %w", err)
	}

	return queryId, nil
}

// checkEvents TODO(oopcode).
func (s *Subscriber) checkEvents(event tmtypes.ResultEvent) (bool, error) {
	events := event.Events

	icqEventsCount := len(events[connectionIdAttr])
	if icqEventsCount == 0 {
		return false, nil
	}

	if len(events[kvKeyAttr]) != icqEventsCount ||
		len(events[transactionsFilterAttr]) != icqEventsCount ||
		len(events[queryIdAttr]) != icqEventsCount ||
		len(events[typeAttr]) != icqEventsCount {
		return false, fmt.Errorf("events attributes length does not match for events=%v", events)
	}

	return true, nil
}

// subscribeQuery returns the subscriber name.
// Note: it doesn't matter what we return here because Tendermint will override it with
// remote IP anyway.
func (s *Subscriber) subscriberName() string {
	return s.targetChainID + "-rpcClient"
}

// subscribeQuery returns a ActiveQuery to filter out interchain ActiveQuery events.
func (s *Subscriber) subscribeQueryUpdated() string {
	return fmt.Sprintf("%s='%s' AND %s='%s' AND %s='%s'",
		connectionIdAttr, s.targetConnectionID,
		moduleAttr, neutrontypes.ModuleName,
		actionAttr, neutrontypes.AttributeValueQueryUpdated,
	)
}

// subscribeQuery returns a ActiveQuery to filter out interchain ActiveQuery events.
func (s *Subscriber) subscribeQueryRemoved() string {
	return fmt.Sprintf("%s='%s' AND %s='%s' AND %s='%s' AND %s='%s'",
		connectionIdAttr, s.targetConnectionID,
		moduleAttr, neutrontypes.ModuleName,
		actionAttr, neutrontypes.AttributeValueQueryRemoved,
	)
}

// subscribeQuery returns a ActiveQuery to filter out interchain ActiveQuery events.
func (s *Subscriber) subscribeQueryBlock() string {
	return fmt.Sprintf("%s='%s'",
		eventAttr, types.EventNewBlockHeader,
	)
}

// isWatchedMsgType returns true if the given message type was added to the subscriber's watched
// ActiveQuery types list.
func (s *Subscriber) isWatchedMsgType(msgType neutrontypes.InterchainQueryType) bool {
	_, ex := s.watchedTypes[msgType]
	return ex
}

// isWatchedAddress returns true if the address is within the registry watched addresses or there
// are no registry watched addresses configured for the subscriber meaning all addresses are watched.
func (s *Subscriber) isWatchedAddress(address string) bool {
	return s.registry.IsEmpty() || s.registry.Contains(address)
}

// extractBlockHeight TODO(oopcode).
func (s *Subscriber) extractBlockHeight(event tmtypes.ResultEvent) (uint64, error) {
	// TODO(oopcode): implement
	return 0, nil
}
