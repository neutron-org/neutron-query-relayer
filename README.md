# Description
Interchain query relayer implementation for Neutron

Makes interchain queries possible:
1. Neutron manages interchain query registration;
2. Relayer sees incoming ICQ events from Neutron;
3. On each event, relayer gets proofs for all the needed data for query from the target chain;
4. Relayer either sends query result to Neutron (for KV queries) or calls query owner's sudo handler (for TX queries if callback execution is allowed by relayer's configuration).

## Running in development
- export environment you need (e.g. `export $(grep -v '^#' .env.example | xargs)` note: change rpc addresses to actual)
- `make dev`

For more configuration parameters see [env parameters section](###Common).

## Testing

### Run unit tests
`$ make test`

### Testing with 2 neutron-chains (easier for development) via cli

#### prerequisites
Clone the following repositories to the same folder where the neutron-query-relayer folder is located:
1. `git clone git@github.com:neutron-org/neutron.git`
2. `git clone git@github.com:neutron-org/neutron-contracts.git`
3. `git clone git@github.com:neutron-org/neutron-integration-tests.git` for testing using docker

#### terminal 1
1. `cd neutron`
2. `make build && make init && make start-rly`

#### terminal 2
1. `cd neutron-contracts`
2. run test_*.sh files from the root of the neutron-contracts project (e.g. `./test_tx_query_result.sh`).

#### terminal 3
1. copy `.env.example` and rename the copy to `.env`
2. set env from env list via way you prefer and run relayer:

`export $(grep -v '^#' .env | xargs) && make dev`

### Testing via docker
1. `cd neutron-integration-tests`
2. read and run preparation steps described in the README.md file
3. run tests as described in the README.md file

In case of unexpected behaviour (e.g. tests failure) you can inspect neutron and relayer logs by doing the following:

Neutron:
1. `docker ps`
2. find the neutron container id
3. `docker exec -it neutron_id bash`
4. `cd /opt/neutron/data`
5. observe neutron logs in file `test-1.log`

Relayer:
1. `docker ps`
2. find the relayer container id
3. `docker logs -f relayer_id`

### Logging
We are using [zap.loger](https://github.com/uber-go/zap)
By default, project spawns classical Production logger. so if there is a need to customize it, consider editing envs (see .env.example for examples)

##  Environment Notes
### Common 

| Key                                              | type                                                           | description                                                                                                                                           | optional |
|--------------------------------------------------|----------------------------------------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------|----------|
| `RELAYER_NEUTRON_CHAIN_CHAIN_PREFIX`             | `string`                                                       | chain prefix of neutron chain                                                                                                                         | required |
| `RELAYER_NEUTRON_CHAIN_RPC_ADDR`                 | `string`                                                       | rpc address of neutron chain                                                                                                                          | required |
| `RELAYER_NEUTRON_CHAIN_CHAIN_ID `                | `string`                                                       | neutron chain id                                                                                                                                      | required |
| `RELAYER_NEUTRON_CHAIN_HOME_DIR   `              | `string`                                                       | path to keys directory                                                                                                                                | required |
| `RELAYER_NEUTRON_CHAIN_SIGN_KEY_NAME`            | `string`                                                       | key name                                                                                                                                              | required |
| `RELAYER_NEUTRON_CHAIN_TIMEOUT `                 | `time`                                                         | timeout of neutron chain provider                                                                                                                     | required |
| `RELAYER_NEUTRON_CHAIN_GAS_PRICES`               | `string`                                                       | specifies how much the user is willing to pay per unit of gas, which can be one or multiple denominations of token                                    | required |
| `RELAYER_NEUTRON_CHAIN_GAS_LIMIT`                | `string`                                                       | the maximum price a relayer user is willing to pay for relayer's paid blockchain actions                                                              | required |
| `RELAYER_NEUTRON_CHAIN_GAS_ADJUSTMENT`           | `float`                                                        | used to scale gas up in order to avoid underestimating. For example, users can specify their gas adjustment as 1.5 to use 1.5 times the estimated gas | required |
| `RELAYER_NEUTRON_CHAIN_CONNECTION_ID`            | `string`                                                       | neutron chain connection ID                                                                                                                           | required |
| `RELAYER_NEUTRON_CHAIN_CLIENT_ID `               | `string`                                                       | IBC client ID for an IBC connection between Neutron chain and target chain (where the result was obtained from)                                       | required |
| `RELAYER_NEUTRON_CHAIN_DEBUG `                   | `bool`                                                         | flag to run neutron chain provider in debug mode                                                                                                      | required |
| `RELAYER_NEUTRON_CHAIN_ACCOUNT_PREFIX `          | `string`                                                       | neutron chain account prefix                                                                                                                          | required |
| `RELAYER_NEUTRON_CHAIN_KEYRING_BACKEND`          | `string`                                                       | [see](https://docs.cosmos.network/master/run-node/keyring.html#the-kwallet-backend)                                                                   | required |
| `RELAYER_NEUTRON_CHAIN_OUTPUT_FORMAT`            | `json`  OR `yaml`                                              | target chain provider output format                                                                                                                   | required |
| `RELAYER_NEUTRON_CHAIN_SIGN_MODE_STR `           | `string`                                                       | [see](https://docs.cosmos.network/master/core/transactions.html#signing-transactions) also consider use short variation, e.g. `direct`                | required |
| `RELAYER_TARGET_CHAIN_RPC_ADDR`                  | `string`                                                       | rpc address of target chain                                                                                                                           | required |
| `RELAYER_TARGET_CHAIN_CHAIN_ID `                 | `string`                                                       | target chain id                                                                                                                                       | required |
| `RELAYER_TARGET_CHAIN_ACCOUNT_PREFIX `           | `string`                                                       | target chain account prefix                                                                                                                           | required |
| `RELAYER_TARGET_CHAIN_VALIDATOR_ACCOUNT_PREFIX ` | `string`                                                       | target chain validator account prefix                                                                                                                 | required |
| `RELAYER_TARGET_CHAIN_TIMEOUT `                  | `time`                                                         | timeout of target chain provider                                                                                                                      | required |
| `RELAYER_TARGET_CHAIN_CONNECTION_ID`             | `time`                                                         | target chain connetcion ID                                                                                                                            | required |
| `RELAYER_TARGET_CHAIN_CLIENT_ID `                | `string`                                                       | IBC client ID for an IBC connection between Neutron chain and target chain (where the result was obtained from)                                       | required |
| `RELAYER_TARGET_CHAIN_DEBUG `                    | `bool`                                                         | flag to run target chain provider in debug mode                                                                                                       | required |
| `RELAYER_TARGET_CHAIN_OUTPUT_FORMAT`             | `json`  or `yaml`                                              | target chain provider output format                                                                                                                   | required |
| `RELAYER_REGISTRY_ADDRESSES`                     | `string`                                                       | a list of comma-separated smart-contract addresses for which the relayer processes interchain queries                                                 | required |
| `RELAYER_ALLOW_TX_QUERIES`                       | `bool`                                                         | if true relayer will process tx queries  (if `false`,  relayer will drop them)                                                                        | required |
| `RELAYER_ALLOW_KV_CALLBACKS`                     | `bool`                                                         | if `true`, will pass proofs as sudo callbacks to contracts                                                                                            | required |
| `RELAYER_MIN_KV_UPDATE_PERIOD`                   | `uint`                                                         | minimal period of queries execution and submission (not less than `n` blocks)                                                                         | optional |
| `RELAYER_STORAGE_PATH`                           | `string`                                                       | path to leveldb storage, will be created on given path if doesn't exists <br/> (NOTE: required if `RELAYER_ALLOW_TX_QUERIES` is `true`)               | optional |
| `RELAYER_CHECK_SUBMITTED_TX_STATUS_DELAY`        | `uint`                                                         | delay in milliseconds to wait before transaction is checked for commit status                                                                         | optional |

### Running via docker
-  with local chains use `host.docker.internal` in `RELAYER_NEUTRON_CHAIN_RPC_ADDR` and `RELAYER_TARGET_CHAIN_RPC_ADDR` instead of `localhost`/`127.0.0.1`
- Note that wallet data path is in the root of docker container `RELAYER_TARGET_CHAIN_HOME_DIR=/data/test-2` `RELAYER_NEUTRON_CHAIN_HOME_DIR=/data/test-1`
### Running without docker 
- consider to change  `RELAYER_NEUTRON_CHAIN_RPC_ADDR` & `RELAYER_TARGET_CHAIN_RPC_ADDR` to actual rpc addresses 
- `RELAYER_TARGET_CHAIN_HOME_DIR` `RELAYER_NEUTRON_CHAIN_HOME_DIR` also need to be changed (keys are generated in `terminal 1`)
