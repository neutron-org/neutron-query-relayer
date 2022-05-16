package relayer

import (
	"strings"
)

//func EventToQueries(event coretypes.ResultEvent) []Query {
//source := event.Events["source"]
//connections := event.Events["message.connection_id"]
//chains := event.Events["message.chain_id"]
//queryIds := event.Events["message.query_id"]
//types := event.Events["message.type"]
//params := event.Events["message.parameters"]
//
//items := len(queryIds)

//TODO: figure out what is it?
//result := event.Data.(datatypes.EventDataTx)

//queries := make([]Query, 0, len(queryIds))
//for i := 0; i < items; i++ {
//	queryParams := parseParams(params, queryIds[i])
//	queries = append(queries, Query{source[0], connections[i], chains[i], queryIds[i], types[i], queryParams})
//}

//return queries
//return []Query{}
//}

func parseParams(params []string, queryID string) map[string]string {
	out := map[string]string{}
	for _, p := range params {
		if strings.HasPrefix(p, queryID) {
			parts := strings.SplitN(p, ":", 3)
			out[parts[1]] = parts[2]
		}
	}
	return out
}
