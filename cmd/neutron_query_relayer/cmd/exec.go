package cmd

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	icqhttp "github.com/neutron-org/neutron-query-relayer/internal/http"
)

// ExecCmd represents the exec command
var ExecCmd = &cobra.Command{
	Use: "exec",
}

func init() {
	ExecCmd.PersistentFlags().StringVarP(&urlICQ, UrlFlagName, "u", "http://localhost:9999", "server url")
	ExecCmd.AddCommand(resubmitFailedTx)
	rootCmd.AddCommand(ExecCmd)
}

// resubmitFailedTx represents the resubmit-tx command
var resubmitFailedTx = &cobra.Command{
	Use:   "resubmit-tx <queryID> <transactionHash>",
	Args:  cobra.ExactArgs(2),
	Short: "Resubmit previously unsuccessfully processed transactions",
	RunE: func(cmd *cobra.Command, args []string) error {
		url, err := cmd.Flags().GetString(UrlFlagName)
		if err != nil {
			return err
		}

		client, err := icqhttp.NewICQClient(url)
		if err != nil {
			return fmt.Errorf("failed to get new icq client: %w", err)
		}
		hash := args[1]

		queryID, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("failed to parse queryID: %w", err)
		}

		req := icqhttp.ResubmitRequest{Txs: []icqhttp.ResubmitTx{{
			QueryID: uint64(queryID),
			Hash:    hash,
		}}}

		err = client.ResubmitTxs(req)
		if err != nil {
			return fmt.Errorf("failed to resubmit unsuccessful tx: %w", err)
		}

		fmt.Printf("Tx queryID=%d hash=%s resubmitted successfully", queryID, hash)
		return nil
	},
}
