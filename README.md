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

# Running
`$ make run`

# Testing
`$ make test` // TODO