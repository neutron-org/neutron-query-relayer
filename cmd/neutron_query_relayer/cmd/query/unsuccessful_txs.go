package query

import (
	"fmt"

	"github.com/neutron-org/neutron-query-relayer/internal/webserver"

	"github.com/spf13/cobra"
)

// UnsuccessfulTxs represents the unsuccessful-txs command
var UnsuccessfulTxs = &cobra.Command{
	Use:   "unsuccessful-txs",
	Short: "Query unsuccessfully processed transactions",
	RunE: func(cmd *cobra.Command, args []string) error {
		url, err := cmd.Flags().GetString(UrlFlagName)
		if err != nil {
			return err
		}

		response := get(url, webserver.UnsuccessfulTxsResource)
		fmt.Printf("Unsuccessful txs:\n%s\n", response)

		return nil
	},
}
