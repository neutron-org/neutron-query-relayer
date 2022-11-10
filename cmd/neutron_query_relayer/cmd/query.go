package cmd

import (
	"log"

	"github.com/spf13/cobra"

	"github.com/neutron-org/neutron-query-relayer/cmd/neutron_query_relayer/cmd/queries"
)

var url string

// QueryCmd represents the query command
var QueryCmd = &cobra.Command{
	Use: "query",
}

func init() {
	QueryCmd.PersistentFlags().StringVarP(&url, queries.UrlFlagName, "u", "", "server url")
	err := QueryCmd.MarkPersistentFlagRequired(queries.UrlFlagName)
	if err != nil {
		log.Fatalf("could not initialize query command: %s", err)
	}

	QueryCmd.AddCommand(queries.UnsuccessfulTxs)

	RootCmd.AddCommand(QueryCmd)
}
