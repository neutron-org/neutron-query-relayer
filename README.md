Interchain query relayer implementation for [Neutron](https://github.com/neutron-org/neutron).

More on relayer in [neutron-docs](https://neutron-org.github.io/neutron-docs/relaying/icq-relayer)

# Running in development

### Natively

- export environment you need (e.g. `export $(grep -v '^#' .env.example | xargs)` note: change rpc addresses to actual)
- `make dev`

For more configuration parameters see [Environment section](#Environment).

### In Docker

1. Build docker image 
`make build-docker`
2. Run
`docker run --env-file .env.example -v $PWD/../neutron/data:/data -p 9999:9999 neutron-org/neutron-query-relayer`
   - note: this command uses relative path to mount keys, run this from root path of `neutron-query-relayer`
   - note: with local chains use `host.docker.internal` in `RELAYER_NEUTRON_CHAIN_RPC_ADDR` and `RELAYER_TARGET_CHAIN_RPC_ADDR` instead of `localhost`/`127.0.0.1`
   - note: on Linux machines it is necessary to pass --add-host=host.docker.internal:host-gateway to Docker in order to make container able to access host network

# Testing

### Run unit tests

`$ make test`

### Testing with 2 neutron-chains (easier for development) via cli

#### prerequisites

Clone the following repositories to the same folder where the neutron-query-relayer folder is located:

1. `git clone git@github.com:neutron-org/neutron.git`
2. `git clone git@github.com:neutron-org/neutron-dev-contracts.git`
3. `git clone git@github.com:neutron-org/neutron-integration-tests.git` for testing using docker

#### terminal 1

1. `cd neutron`
2. `make build && make init && make start-rly`

#### terminal 2

1. `cd neutron-dev-contracts`
2. run test_*.sh files from the root of the neutron-dev-contracts project (e.g. `./test_tx_query_result.sh`).

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

# Configuration

### Environment 

| Key                                              | type              | description                                                                                                                                                                | optional |
|--------------------------------------------------|-------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------|
| `RELAYER_NEUTRON_CHAIN_RPC_ADDR`                 | `string`          | rpc address of neutron chain                                                                                                                                               | required |
| `RELAYER_NEUTRON_CHAIN_REST_ADDR`                | `string`          | rest address of neutron chain                                                                                                                                              | required |
| `RELAYER_NEUTRON_CHAIN_HOME_DIR   `              | `string`          | path to keys directory                                                                                                                                                     | required |
| `RELAYER_NEUTRON_CHAIN_SIGN_KEY_NAME`            | `string`          | key name                                                                                                                                                                   | required |
| `RELAYER_NEUTRON_CHAIN_TIMEOUT `                 | `time`            | timeout of neutron chain provider                                                                                                                                          | optional |
| `RELAYER_NEUTRON_CHAIN_GAS_PRICES`               | `string`          | specifies how much the user is willing to pay per unit of gas, which can be one or multiple denominations of token                                                         | required |
| `RELAYER_NEUTRON_CHAIN_GAS_LIMIT`                | `string`          | the maximum price a relayer user is willing to pay for relayer's paid blockchain actions                                                                                   | required |
| `RELAYER_NEUTRON_CHAIN_GAS_ADJUSTMENT`           | `float`           | used to scale gas up in order to avoid underestimating. For example, users can specify their gas adjustment as 1.5 to use 1.5 times the estimated gas                      | required |
| `RELAYER_NEUTRON_CHAIN_CONNECTION_ID`            | `string`          | neutron chain connection ID                                                                                                                                                | required |
| `RELAYER_NEUTRON_CHAIN_DEBUG `                   | `bool`            | flag to run neutron chain provider in debug mode                                                                                                                           | optional |
| `RELAYER_NEUTRON_CHAIN_KEYRING_BACKEND`          | `string`          | [see](https://docs.cosmos.network/master/run-node/keyring.html#the-kwallet-backend)                                                                                        | required |
| `RELAYER_NEUTRON_CHAIN_OUTPUT_FORMAT`            | `json`  OR `yaml` | neutron chain provider output format                                                                                                                                       | required |
| `RELAYER_NEUTRON_CHAIN_SIGN_MODE_STR `           | `string`          | [see](https://docs.cosmos.network/master/core/transactions.html#signing-transactions) also consider use short variation, e.g. `direct`                                     | optional |
| `RELAYER_TARGET_CHAIN_RPC_ADDR`                  | `string`          | rpc address of target chain                                                                                                                                                | required |
| `RELAYER_TARGET_CHAIN_ACCOUNT_PREFIX `           | `string`          | target chain account prefix                                                                                                                                                | required |
| `RELAYER_TARGET_CHAIN_VALIDATOR_ACCOUNT_PREFIX ` | `string`          | target chain validator account prefix                                                                                                                                      | required |
| `RELAYER_TARGET_CHAIN_TIMEOUT `                  | `time`            | timeout of target chain provider                                                                                                                                           | optional |
| `RELAYER_TARGET_CHAIN_DEBUG `                    | `bool`            | flag to run target chain provider in debug mode                                                                                                                            | optional |
| `RELAYER_TARGET_CHAIN_OUTPUT_FORMAT`             | `json`  or `yaml` | target chain provider output format                                                                                                                                        | optional |
| `RELAYER_REGISTRY_ADDRESSES`                     | `string`          | a list of comma-separated smart-contract addresses for which the relayer processes interchain queries                                                                      | required |
| `RELAYER_ALLOW_TX_QUERIES`                       | `bool`            | if true relayer will process tx queries  (if `false`, relayer will drop them)                                                                                              | required |
| `RELAYER_ALLOW_KV_CALLBACKS`                     | `bool`            | if `true`, will pass proofs as sudo callbacks to contracts                                                                                                                 | required |
| `RELAYER_MIN_KV_UPDATE_PERIOD`                   | `uint`            | minimal period of queries execution and submission (not less than `n` blocks)                                                                                              | optional |
| `RELAYER_STORAGE_PATH`                           | `string`          | path to leveldb storage, will be created on given path if doesn't exists <br/> (required if `RELAYER_ALLOW_TX_QUERIES` is `true`)                                          | optional |
| `RELAYER_CHECK_SUBMITTED_TX_STATUS_DELAY`        | `uint`            | delay in seconds to wait before transaction is checked for commit status                                                                                                   | optional |
| `RELAYER_QUERIES_TASK_QUEUE_CAPACITY`            | `int`             | capacity of the channel that is used to send messages from subscriber to relayer (better set to a higher value to avoid problems with Tendermint websocket subscriptions). | optional |
| `RELAYER_PROMETHEUS_PORT`                        | `uint`            | listen port for Prometheus                                                                                                                                                 | optional |
| `RELAYER_INITIAL_TX_SEARCH_OFFSET`               | `uint`            | if set to non zero and no prior search height exists, it will initially set to (last_height - X). Set this if you have lots of old tx's on first start you don't need.     | optional |

# Logging

We are using a little modified [zap.Logger](https://github.com/uber-go/zap), the modification can be seen at the [neutron-logger repository](https://github.com/neutron-org/neutron-logger). The default version of the logger used in the application is a little bit modified [zap.NewProduction](https://github.com/uber-go/zap/blob/master/logger.go#L94). The logger level can be set via the `LOGGER_LEVEL` env variable. If there is a need for a more significant customisation of the logger behaviour, see the neutron-logger repository readme.

## Generate Openapi

The relayer uses a generated swagger client to retrieve queries information from Neutron. You can re-generate the Go
client from `./internal/subscriber/querier/openapi.yml` by running:

```
make generate-openapi
```

The swagger specs can be taken from the Neutron [repo](https://github.com/neutron-org/neutron/blob/main/docs/static/openapi.yml) (you need to remove most of the generated specification). 
