package proofer

import (
	"github.com/tendermint/tendermint/crypto/merkle"
	"github.com/tendermint/tendermint/proto/tendermint/crypto"
	coretypes "github.com/tendermint/tendermint/types"
)

// TODO: use types from repo?
// StorageValue is basic building proof block
type StorageValue struct {
	StoragePrefix string
	Key           []byte
	Value         []byte
	Proofs        []crypto.ProofOp
}

// TODO: are these types enough for our lido chain?
type CompleteTransactionProof struct {
	BlockProof   coretypes.TxProof
	SuccessProof merkle.Proof
	Height       uint64
}

// Query has all the information for what we can collect Proof
type Query struct {
	SourceChainId string
	ConnectionId  string
	ChainId       string
	QueryId       string
	Type          string
	Params        map[string]string
}
