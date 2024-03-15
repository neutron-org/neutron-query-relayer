package subscriber

import (
	"github.com/neutron-org/neutron/x/interchainqueries/types"
)

const eventTypePrefix = types.EventTypeNeutronMessage

// types of keys for parsing incoming events
const (
	// moduleAttr is the key of the message event's attribute that contains the module the event
	// originates from.
	moduleAttr = eventTypePrefix + ".module"
	// actionAttr is the key of the message event's attribute that contains the event's action.
	actionAttr = eventTypePrefix + ".action"
	// eventAttr is the key of the tendermint's event attribute that contains the kind of event.
	eventAttr = "tm.event"

	// ConnectionIdAttr is the key of the Neutron's custom message event's attribute that contains the
	// connectionID of the event's ActiveQuery.
	ConnectionIdAttr = eventTypePrefix + "." + types.AttributeKeyConnectionID
	// QueryIdAttr is the key of the Neutron's custom message event's attribute that contains the
	// incoming ICQ ID.
	QueryIdAttr = eventTypePrefix + "." + types.AttributeKeyQueryID
	// KvKeyAttr is the key of the Neutron's custom message event's attribute that contains the KV
	// values for the incoming KV ICQ.
	KvKeyAttr = eventTypePrefix + "." + types.AttributeKeyKVQuery
	// TransactionsFilterAttr is the key of the Neutron's custom message event's attribute that
	// contains the transaction filter value for the incoming TX ICQ.
	TransactionsFilterAttr = eventTypePrefix + "." + types.AttributeTransactionsFilterQuery
	// TypeAttr is the key of the Neutron's custom message event's attribute that contains the type
	// of the incoming ICQ.
	TypeAttr = eventTypePrefix + "." + types.AttributeKeyQueryType
	// OwnerAttr is the key of the Neutron's custom message event's attribute that contains the
	// address of the ICQ owner.
	OwnerAttr = eventTypePrefix + "." + types.AttributeKeyOwner
)
