package proof_impl

import (
	"encoding/json"
	"github.com/lidofinance/cosmos-query-relayer/internal/proof"
	"os"
)

type marshallingStorageValue struct {
	StoragePrefix string
	Key           []byte
	Value         []byte
	Proofs        []marshallingCryptoOp
}

type marshallingCryptoOp struct {
	Type string
	Key  []byte
	Data []byte
}

func toMarshallingType(value proof.StorageValue) marshallingStorageValue {
	proofs := make([]marshallingCryptoOp, 0, len(value.Proofs))
	for _, item := range value.Proofs {
		proofs = append(proofs, marshallingCryptoOp{
			Type: item.Type,
			Key:  item.Key,
			Data: item.Data,
		})
	}
	return marshallingStorageValue{
		StoragePrefix: value.StoragePrefix,
		Key:           value.Key,
		Value:         value.Value,
		Proofs:        proofs,
	}
}

func marshalStorageValueToJson(value []proof.StorageValue) ([]byte, error) {
	res := make([]marshallingStorageValue, 0, len(value))
	for _, item := range value {
		m := toMarshallingType(item)
		res = append(res, m)
	}
	return json.Marshal(res)
}

func unmarshalJsonToStorage(string) []proof.StorageValue {
	return nil
}

func getCurrentDir() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	return dir, nil
}
