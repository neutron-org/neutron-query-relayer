/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>
*/
package query

import (
	"fmt"

	"github.com/neutron-org/neutron-query-relayer/cmd/neutron_query_relayer/cmd"

	"github.com/spf13/cobra"
)

// UnsuccessfulTxs represents the unsuccessful-txs command
var UnsuccessfulTxs = &cobra.Command{
	Use:   "unsuccessful-txs",
	Short: "Query unsuccessfully processed transactions",
	RunE: func(_ *cobra.Command, args []string) error {
		fmt.Println("Unsuccessful txs")
		url, err := UnsuccessfulTxs.Flags().GetString(cmd.UrlFlagName)
		fmt.Printf("url: %s", url)

		return nil
	},
}

func init() {
	cmd.QueryCmd.AddCommand(UnsuccessfulTxs)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// versionCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// versionCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
