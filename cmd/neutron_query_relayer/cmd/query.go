/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"github.com/spf13/cobra"
)

// TODO: rename to url
const UrlFlagName = "node"

var url string

// QueryCmd represents the query command
var QueryCmd = &cobra.Command{
	Use: "query",
	//Short: "Query information about relayer",
	//Long:  `Query information about relayers work like unprocessed transactions.`,
}

func init() {
	QueryCmd.Flags().StringVarP(&url, UrlFlagName, "n", "", "url server url")
	//err := QueryCmd.MarkFlagRequired(UrlFlagName)
	//if err != nil {
	//	log.Fatalf("could not initialize query command: %s", err)
	//}

	RootCmd.AddCommand(QueryCmd)
}
