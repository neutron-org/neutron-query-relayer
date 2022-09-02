package relay

import neutrontypes "github.com/neutron-org/neutron/x/interchainqueries/types"

type queryEventMessage struct {
	queryId            uint64
	messageType        neutrontypes.InterchainQueryType
	kvKeys             neutrontypes.KVKeys
	transactionsFilter string
}

type RecipientTransactionsParams []struct {
	Field string
	Op    string
	Value interface{}
}

// types of keys for parsing incoming events
const (
	connectionIdAttr   = "message." + neutrontypes.AttributeKeyConnectionID
	queryIdAttr        = "message." + neutrontypes.AttributeKeyQueryID
	kvKeyAttr          = "message." + neutrontypes.AttributeKeyKVQuery
	transactionsFilter = "message." + neutrontypes.AttributeTransactionsFilterQuery
	typeAttr           = "message." + neutrontypes.AttributeKeyQueryType
	ownerAttr          = "message." + neutrontypes.AttributeKeyOwner
)
