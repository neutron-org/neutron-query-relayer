/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>
*/
package query

import (
	"fmt"

	"github.com/neutron-org/neutron-query-relayer/cmd/neutron_query_relayer/cmd"
	"github.com/spf13/cobra"
)

// versionCmd represents the version command
var unsuccessfulTxs = &cobra.Command{
	Use:   "unsuccessful-txs",
	Short: "Query unsuccessfully processed transactions",
	RunE: func(cmd *cobra.Command, args []string) error {
		// TODO: defer closing of storage access
		//cfg, err := config.NewNeutronQueryRelayerConfig()
		//if err != nil {
		//	return fmt.Errorf("could not initialize config: %w", cfg)
		//	// TODO: end command with error
		//	//logger.Fatal("cannot initialize relayer config", zap.Error(err))
		//}
		//
		//if !cfg.AllowTxQueries {
		//	return fmt.Errorf("transaction queries are not allowed in config")
		//}
		//
		//if cfg.StoragePath == "" {
		//	return fmt.Errorf("storage path is empty")
		//}
		//
		//store, err := storage.NewLevelDBStorage(cfg.StoragePath)
		//if err != nil {
		//	return fmt.Errorf("couldn't initialize levelDB storage: %w", err)
		//}
		//
		//failedTxs := store.GetAllPendingTxs()
		//
		//return nil

		fmt.Println("Unsuccessful txs")

		return nil
	},
}

func init() {
	cmd.RootCmd.AddCommand(unsuccessfulTxs)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// versionCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// versionCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
