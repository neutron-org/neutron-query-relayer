/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>
*/
package query

import (
	"fmt"

	"github.com/spf13/cobra"
)

const resource = "/unsuccessful-txs"

// UnsuccessfulTxs represents the unsuccessful-txs command
var UnsuccessfulTxs = &cobra.Command{
	Use:   "unsuccessful-txs",
	Short: "Query unsuccessfully processed transactions",
	RunE: func(cmd *cobra.Command, args []string) error {
		url, err := cmd.Flags().GetString(UrlFlagName)
		if err != nil {
			return err
		}
		fmt.Printf("url: %s", url)

		response := get(url, resource)
		fmt.Printf("Unsuccessful txs:\n%s\n", response)

		return nil
	},
}
