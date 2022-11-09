package cmd

import (
	"log"

	"github.com/neutron-org/neutron-query-relayer/cmd/neutron_query_relayer/cmd/query"

	"github.com/spf13/cobra"
)

var url string

// QueryCmd represents the query command
var QueryCmd = &cobra.Command{
	Use: "query",
}

func init() {
	QueryCmd.PersistentFlags().StringVarP(&url, query.UrlFlagName, "u", "", "server url")
	err := QueryCmd.MarkPersistentFlagRequired(query.UrlFlagName)
	if err != nil {
		log.Fatalf("could not initialize query command: %s", err)
	}

	QueryCmd.AddCommand(query.UnsuccessfulTxs)

	RootCmd.AddCommand(QueryCmd)
}
