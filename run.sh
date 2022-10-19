#!/bin/bash
NODE=${NODE:-node}
echo "Waiting for a first block..."
while ! curl -f ${NODE}:1317/blocks/1 >/dev/null 2>&1; do
  sleep 1
done
echo "Start relayer"
neutron_query_relayer