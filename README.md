# Description
Interchain query relayer implementation for Cosmos

Makes interchain queries possible.
For example there is blockchain L that needs to make query to blockchain T.
L -> T

Blockchain L submits an interchain query with needed params and so on.

Relayer sees the incoming event from blockchain L and:
1. Tries to parse it from list of supported queries
2. If successful, gets proofs for all the needed data for query
3. If successful, submits transaction with proofs back to blockchain L

Blockchain L can then verify the result for the query.

# Running in development
- `$ cp configs/dev.example.yml configs/dev.yml`
- Fill your configs/dev.yml with necessary values (see, sign-key-name and keyring-dir)
- `$ make dev`

For more configuration parameters see struct in internal/config/config.go

# Testing
`$ make test`

# Testing with 2 neutron-chains (easier for development)

### terminal 1

1. `git clone git@github.com:neutron-org/neutron.git`
2. `cd neutron`
3. `make build && make init && make start-rly`

### terminal 2
see test-2/config/genesis.json for $VAL2 value

1. Create delegation from demowallet2 to val2 on test-2 chain
```
VAL2=neutronvaloper1qnk2n4nlkpw9xfqntladh74w6ujtulwnqshepx`
DEMOWALLET2=$(neutrond keys show demowallet2 -a --keyring-backend test --home ./data/test-2)
echo "DEMOWALLET2: $DEMOWALLET2
./build/neutrond tx staking delegate $VAL2 1stake --from demowallet2 --keyring-backend test --home ./data/test-2 --chain-id=test-2 -y
```
2. Register interchain query
`./build/neutrond tx interchainqueries register-interchain-query test-2 connection-0 x/staking/DelegatorDelegations '{"delegator": "neutron10h9stc5v6ntgeygf5xf945njqq5h32r54rf7kf"}' 1 --from demowallet1 --gas 10000000 --gas-adjustment 1.4 --gas-prices 0.5stake --broadcast-mode block --chain-id test-1 --keyring-backend test --home ./data/test-1 --node tcp://127.0.0.1:16657`

### terminal 3

1. `cp configs/dev.example.2-neutron-chains.yml configs/dev.yml`
2. `make dev`
