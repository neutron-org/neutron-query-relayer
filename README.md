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

# Testing with 2 lido-chains (easier for development)

### terminal 1

1. `git clone git@github.com:lidofinance/gaia-wasm-zone.git`
2. `cd gaia-wasm-zone`
3. `make init`
4. `make start-rly`

### terminal 2

1. `gaia-wasm-zoned tx interchainqueries register-interchain-query test-2 connection-0 x/staking/DelegatorDelegations '{"delegator": "cosmos1qnk2n4nlkpw9xfqntladh74w6ujtulwn7j8za9"}' 1 --from demowallet1 --gas 10000000 --gas-adjustment 1.4 --gas-prices 0.5stake --broadcast-mode block --chain-id test-1 --keyring-backend test --home ./data/test-1 --node tcp://127.0.0.1:16657`

### terminal 3

1. `cp configs/dev.example.2-lido-chains.yml configs/dev.yml`
2. `make dev`
