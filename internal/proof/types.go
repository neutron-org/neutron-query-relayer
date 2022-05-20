package proof

import (
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto/merkle"
	"github.com/tendermint/tendermint/proto/tendermint/crypto"
)

// StorageValue is basic building proof block
type StorageValue struct {
	StoragePrefix string
	Key           []byte
	Value         []byte
	Proofs        []crypto.ProofOp
}

// TxValue is basic building tx proof block
type TxValue struct {
	InclusionProof merkle.Proof
	DeliveryProof  merkle.Proof
	Height         uint64
	Tx             *abci.ResponseDeliverTx
}
