package subscriber

import "github.com/neutron-org/neutron/x/interchainqueries/types"

// types of keys for parsing incoming events
const (
	// moduleAttr is the key of the message event's attribute that contains the module the event
	// originates from.
	moduleAttr = "neutron.module"
	// actionAttr is the key of the message event's attribute that contains the event's action.
	actionAttr = "neutron.action"
	// eventAttr is the key of the tendermint's event attribute that contains the kind of event.
	eventAttr = "tm.event"

	// connectionIdAttr is the key of the ToNeutronRegisteredQuery's custom message event's attribute that contains the
	// connectionID of the event's ActiveQuery.
	connectionIdAttr = "neutron." + types.AttributeKeyConnectionID
	// queryIdAttr is the key of the ToNeutronRegisteredQuery's custom message event's attribute that contains the
	// incoming ICQ ID.
	queryIdAttr = "neutron." + types.AttributeKeyQueryID
	// kvKeyAttr is the key of the ToNeutronRegisteredQuery's custom message event's attribute that contains the KV
	// values for the incoming KV ICQ.
	kvKeyAttr = "neutron." + types.AttributeKeyKVQuery
	// transactionsFilterAttr is the key of the ToNeutronRegisteredQuery's custom message event's attribute that
	// contains the transaction filter value for the incoming TX ICQ.
	transactionsFilterAttr = "neutron." + types.AttributeTransactionsFilterQuery
	// typeAttr is the key of the ToNeutronRegisteredQuery's custom message event's attribute that contains the type
	// of the incoming ICQ.
	typeAttr = "neutron." + types.AttributeKeyQueryType
	// ownerAttr is the key of the ToNeutronRegisteredQuery's custom message event's attribute that contains the
	// address of the ICQ owner.
	ownerAttr = "neutron." + types.AttributeKeyOwner
)
