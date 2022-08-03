package relay

import neutrontypes "github.com/neutron-org/neutron/x/interchainqueries/types"

type queryEventMessage struct {
	queryId     uint64
	messageType string
	parameters  []byte
}

type delegatorDelegationsParams struct {
	Delegator string `json:"delegator"`
}

type getBalanceParams struct {
	Addr  string `json:"addr"`
	Denom string `json:"denom"`
}

type recipientTransactionsParams map[string]string

type exchangeRateParams struct {
	// GetSupply part
	Denom string `json:"denom"`

	// GetDelegatorDelegations part
	Delegator string `json:"delegator"`
}

// types of keys for parsing incoming events
const (
	zoneIdAttr     = "message." + neutrontypes.AttributeKeyZoneID
	queryIdAttr    = "message." + neutrontypes.AttributeKeyQueryID
	parametersAttr = "message." + neutrontypes.AttributeKeyQueryParameters
	typeAttr       = "message." + neutrontypes.AttributeKeyQueryType
	ownerAttr      = "message." + neutrontypes.AttributeKeyOwner
)

// types of incoming query messages
const (
	delegatorDelegationsType  = "x/staking/DelegatorDelegations"
	getBalanceType            = "x/bank/GetBalance"
	exchangeRateType          = "x/bank/ExchangeRate"
	recipientTransactionsType = "x/tx/RecipientTransactions"
	delegationRewardsType     = "x/distribution/CalculateDelegationRewards"
)
