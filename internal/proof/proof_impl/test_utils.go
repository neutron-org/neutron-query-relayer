package proof_impl

import (
	"encoding/json"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/lidofinance/cosmos-query-relayer/internal/proof"
	abci "github.com/tendermint/tendermint/abci/types"
)

import (
	"context"
	"fmt"
	"github.com/lidofinance/cosmos-query-relayer/internal/config"
	raw "github.com/lidofinance/cosmos-query-relayer/internal/raw"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	"log"
	"os"
)

func PrefillDelegations(ctx context.Context, cfg config.CosmosQueryRelayerConfig) {
	client, err := raw.NewRPCClient(cfg.TargetChain.RPCAddress, cfg.TargetChain.Timeout)
	if err != nil {
		err = fmt.Errorf("error creating new http client: %w", err)
		log.Println(err)
	}
	querier, err := proof.NewQuerier(client, cfg.TargetChain.ChainID)
	if err != nil {
		err = fmt.Errorf("error creating new query key proofer: %w", err)
		log.Println(err)
	}
	proofer := NewProofer(querier)
	delegations, _, err := proofer.GetDelegatorDelegations(ctx, 0, "terra", "terra1mtwph2juhj0rvjz7dy92gvl6xvukaxu8rfv8ts")
	jsonBytes, err := MarshalStorageValueToJson(delegations)
	if err != nil {
		err = fmt.Errorf("error marshalling json: %w", err)
		log.Println(err)
	}
	//fmt.Printf("\nJSON for height=%d: \n%s\n\n", height, jsonBytes)

	dir, err := getCurrentDir()
	log.Printf("Dir: %s", dir)
	f, err := os.Create("tests/test_data/get_delegator_delegation.json")

	if err != nil {
		log.Fatal(err)
	}

	defer f.Close()

	_, err2 := f.Write(jsonBytes)

	if err2 != nil {
		log.Fatal(err2)
	}

	response, err := queryDelegations(context.Background(), client, "terra1mtwph2juhj0rvjz7dy92gvl6xvukaxu8rfv8ts")
	if err != nil {
		log.Fatalln(err)
	}

	for _, item := range response {
		fmt.Printf("Delegation: DelegatorAddress= %s Shares=%+v\n", item.Delegation.DelegatorAddress, item.Delegation.Shares)
	}
}

func getCurrentDir() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	return dir, nil
}

func queryDelegations(ctx context.Context, rpcClient rpcclient.Client, address string) ([]stakingtypes.DelegationResponse, error) {
	request := &stakingtypes.QueryDelegatorDelegationsRequest{DelegatorAddr: address}
	req, err := request.Marshal()
	if err != nil {
		return nil, err
	}
	simQuery := abci.RequestQuery{
		Path: "/cosmos.staking.v1beta1.Query/DelegatorDelegations",
		Data: req,
	}
	res, err := rpcClient.ABCIQueryWithOptions(ctx, simQuery.Path, simQuery.Data, rpcclient.DefaultABCIQueryOptions)
	if err != nil {
		return nil, err
	}
	log.Printf("LOG: %s\n", res.Response.Log)

	if res.Response.Code != 0 {
		return nil, fmt.Errorf("error fetching account with address=%s log=%s", address, res.Response.Log)
	}

	var response stakingtypes.QueryDelegatorDelegationsResponse
	if err := response.Unmarshal(res.Response.Value); err != nil {
		return nil, err
	}

	return response.DelegationResponses, nil
}

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

func MarshalStorageValueToJson(value []proof.StorageValue) ([]byte, error) {
	res := make([]marshallingStorageValue, 0, len(value))
	for _, item := range value {
		m := toMarshallingType(item)
		res = append(res, m)
	}
	return json.Marshal(res)
}

func UnmarshalJsonToStorage(string) []proof.StorageValue {
	return nil
}
