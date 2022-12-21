package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	icqhttp "github.com/neutron-org/neutron-query-relayer/internal/http"
)

var urlICQ string

const (
	UrlFlagName = "url"
)

// QueryCmd represents the query command
var QueryCmd = &cobra.Command{
	Use: "query",
}

func init() {
	QueryCmd.PersistentFlags().StringVarP(&urlICQ, UrlFlagName, "u", "http://localhost:9999", "server url")
	QueryCmd.AddCommand(UnsuccessfulTxs)
	rootCmd.AddCommand(QueryCmd)
}

// UnsuccessfulTxs represents the unsuccessful-txs command
var UnsuccessfulTxs = &cobra.Command{
	Use:   "unsuccessful-txs",
	Short: "Query unsuccessfully processed transactions",
	RunE: func(cmd *cobra.Command, args []string) error {
		url, err := cmd.Flags().GetString(UrlFlagName)
		if err != nil {
			return err
		}

		client, err := icqhttp.NewICQClient(url)
		if err != nil {
			return fmt.Errorf("failed to get new icq client: %w", err)
		}

		txs, err := client.GetUnsuccessfulTxs()
		if err != nil {
			return fmt.Errorf("failed to get unsuccessful txs: %w", err)
		}

		var response bytes.Buffer
		encoder := json.NewEncoder(&response)
		encoder.SetIndent("", "  ")
		err = encoder.Encode(txs)
		if err != nil {
			return fmt.Errorf("failed to encode unsuccessful transactions: %w", err)
		}

		fmt.Printf("Unsuccessful txs:\n%s\n", response.String())

		return nil
	},
}
