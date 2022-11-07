package cmd

import (
	"fmt"

	"github.com/neutron-org/neutron-query-relayer/internal/app"
	"github.com/spf13/cobra"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Version on the query relayer",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Version:", app.Version)
		fmt.Println("Commit:", app.Commit)
	},
}

func init() {
	RootCmd.AddCommand(versionCmd)
}
