/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"log"

	"github.com/neutron-org/neutron-query-relayer/cmd/neutron_query_relayer/cmd/query"

	"github.com/spf13/cobra"
)

const UrlFlagName = "url"

var url string

// QueryCmd represents the query command
var QueryCmd = &cobra.Command{
	Use:              "query",
	TraverseChildren: true,
	//Short: "Query information about relayer",
	//Long:  `Query information about relayers work like unprocessed transactions.`,
}

func init() {
	QueryCmd.PersistentFlags().StringVarP(&url, UrlFlagName, "u", "", "server url")
	err := QueryCmd.MarkPersistentFlagRequired(UrlFlagName)
	if err != nil {
		log.Fatalf("could not initialize query command: %s", err)
	}
	QueryCmd.AddCommand(query.UnsuccessfulTxs)

	RootCmd.AddCommand(QueryCmd)
}
