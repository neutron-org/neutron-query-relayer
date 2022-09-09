package subscriber

import (
	"context"
	"fmt"
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
	client, err := http.New(rpcAddress, "/websocket")
	if err != nil {
		return nil, fmt.Errorf("could not create new tendermint client: %w", err)
	}

	if err = client.Start(); err != nil {
		return nil, fmt.Errorf("could not start tendermint client: %w", err)
	}

	watchedTypesMap := make(map[neutrontypes.InterchainQueryType]struct{})
	for _, queryType := range watchedTypes {
		watchedTypesMap[queryType] = struct{}{}
	}

	return &Subscriber{
		client:             client,
		rpcAddress:         rpcAddress,
		targetChainID:      targetChainID,
		targetConnectionID: targetConnectionID,
		registry:           registry,
		logger:             logger,
		watchedTypes:       watchedTypesMap,

		activeQueries: map[uint64]*neutrontypes.RegisteredQuery{},
	}, nil
}

// Subscriber is responsible for subscribing on chain's ICQ events. It parses incoming events,
// filters them in accordance with the Registry configuration and watchedTypes, and provides a
// stream of split to KV and TX messages.
type Subscriber struct {
	client             *http.HTTP
	rpcAddress         string
	targetChainID      string
	targetConnectionID string
	registry           *registry.Registry
	logger             *zap.Logger
	watchedTypes       map[neutrontypes.InterchainQueryType]struct{}

	activeQueries map[uint64]*neutrontypes.RegisteredQuery
}

// loadActiveQueries TODO(oopcode).
func (s *Subscriber) loadActiveQueries() error {
	// TODO(oopcode): implement
	return nil
}

// Subscribe TODO(oopcode).
func (s *Subscriber) Subscribe(ctx context.Context, tasks *queue.Queue[neutrontypes.RegisteredQuery]) error {
	if err := s.loadActiveQueries(); err != nil {
		return fmt.Errorf("could not loadActiveQueries: %w", err)
	}

	updateEvents, err := s.client.Subscribe(ctx, s.subscriberName(), s.subscribeQueryUpdated())
	if err != nil {
		return fmt.Errorf("could not subscribe to events: %w", err)
	}

	removeEvents, err := s.client.Subscribe(ctx, s.subscriberName(), s.subscribeQueryRemoved())
	if err != nil {
		return fmt.Errorf("could not subscribe to events: %w", err)
	}

	blockEvents, err := s.client.Subscribe(ctx, s.subscriberName(), s.subscribeQueryBlock())
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
			if err := s.processUpdateEvent(event, tasks); err != nil {
				return fmt.Errorf("failed to processUpdateEvent: %w", err)
			}
		case event := <-removeEvents:
			if err := s.processRemoveEvent(event, tasks); err != nil {
				return fmt.Errorf("failed to processRemoveEvent: %w", err)
			}
		}
	}
}

func (s *Subscriber) unsubscribe() {
	ctx, cancel := context.WithTimeout(context.Background(), unsubscribeTimeout)
	defer cancel()

	if err := s.client.Unsubscribe(ctx, s.subscriberName(), s.subscribeQueryUpdated()); err != nil {
		s.logger.Error("failed to Unsubscribe from tm events",
			zap.Error(err), zap.String("ActiveQuery", s.subscribeQueryUpdated()))
	}

	if err := s.client.Unsubscribe(ctx, s.subscriberName(), s.subscribeQueryRemoved()); err != nil {
		s.logger.Error("failed to Unsubscribe from tm events",
			zap.Error(err), zap.String("ActiveQuery", s.subscribeQueryRemoved()))
	}

	if err := s.client.Unsubscribe(ctx, s.subscriberName(), s.subscribeQueryBlock()); err != nil {
		s.logger.Error("failed to Unsubscribe from tm events",
			zap.Error(err), zap.String("ActiveQuery", s.subscribeQueryBlock()))
	}
}

func (s *Subscriber) processBlockEvent(event tmtypes.ResultEvent, tasks *queue.Queue[neutrontypes.RegisteredQuery]) error {
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
func (s *Subscriber) processUpdateEvent(event tmtypes.ResultEvent, tasks *queue.Queue[neutrontypes.RegisteredQuery]) error {
	// TODO(oopcode): implement
	return nil
}

// processRemoveEvent TODO(oopcode).
func (s *Subscriber) processRemoveEvent(event tmtypes.ResultEvent, tasks *queue.Queue[neutrontypes.RegisteredQuery]) error {
	// TODO(oopcode): implement
	return nil
}

// extractBlockHeight TODO(oopcode).
func (s *Subscriber) extractBlockHeight(event tmtypes.ResultEvent) (uint64, error) {
	// TODO(oopcode): implement
	return 0, nil
}

// subscribeQuery returns the subscriber name.
// Note: it doesn't matter what we return here because Tendermint will override it with
// remote IP anyway.
func (s *Subscriber) subscriberName() string {
	return s.targetChainID + "-client"
}

// subscribeQuery returns a ActiveQuery to filter out interchain ActiveQuery events.
func (s *Subscriber) subscribeQueryUpdated() string {
	return fmt.Sprintf("%s='%s' AND %s='%s' AND %s='%s' AND %s='%s'",
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
	return fmt.Sprintf("%s='%s' AND %s='%s' AND %s='%s' AND %s='%s'",
		connectionIdAttr, s.targetConnectionID,
		moduleAttr, neutrontypes.ModuleName,
		eventAttr, types.EventNewBlockHeader,
	)
}

// extractMessages validates events received from the target chain, filters them, and transforms
// to a slice of queryEventMessage.
func (s *Subscriber) extractMessages(e tmtypes.ResultEvent) ([]*queryEventMessage, error) {
	events := e.Events
	icqEventsCount := len(events[connectionIdAttr])
	if icqEventsCount == 0 {
		return nil, nil
	}

	if len(events[kvKeyAttr]) != icqEventsCount ||
		len(events[transactionsFilterAttr]) != icqEventsCount ||
		len(events[queryIdAttr]) != icqEventsCount ||
		len(events[typeAttr]) != icqEventsCount {
		return nil, fmt.Errorf("events attributes length does not match for events=%v", events)
	}

	messages := make([]*queryEventMessage, 0, icqEventsCount)
	for idx := range events[connectionIdAttr] {
		if !s.isWatchedAddress(events[ownerAttr][idx]) {
			continue
		}

		queryIdStr := events[queryIdAttr][idx]
		queryId, err := strconv.ParseUint(queryIdStr, 10, 64)
		if err != nil {
			s.logger.Debug("failed to parse ActiveQuery ID as uint", zap.String("query_id", queryIdStr), zap.Error(err))
			continue
		}

		messageType := neutrontypes.InterchainQueryType(events[typeAttr][idx])
		switch messageType {
		case neutrontypes.InterchainQueryTypeKV:
			kvKeyAttrVal := events[kvKeyAttr][idx]
			kvKeys, err := neutrontypes.KVKeysFromString(kvKeyAttrVal)
			if err != nil {
				s.logger.Debug(fmt.Sprintf("invalid %s attr", kvKeyAttr), zap.String(kvKeyAttr, kvKeyAttrVal), zap.Error(err))
				continue
			}
			messages = append(messages, &queryEventMessage{
				queryId:     queryId,
				messageType: messageType,
				kvKeys:      kvKeys,
			})
		case neutrontypes.InterchainQueryTypeTX:
			messages = append(messages, &queryEventMessage{
				queryId:            queryId,
				messageType:        messageType,
				transactionsFilter: events[transactionsFilterAttr][idx],
			})
		default:
			s.logger.Debug("unknown query_type", zap.String("query_type", string(messageType)))
		}
	}
	return messages, nil
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
