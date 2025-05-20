#!/bin/bash
NODE=${NODE:-node}
echo "Waiting for first 10 blocks on $NODE..."
while ! curl -f ${NODE}:1317/cosmos/base/tendermint/v1beta1/blocks/10 ; do
  echo
  sleep 1
done
echo
echo "Start relayer"
neutron_query_relayer start
