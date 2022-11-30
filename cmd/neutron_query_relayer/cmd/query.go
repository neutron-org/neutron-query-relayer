package cmd

import (
	"bytes"
	"fmt"
	"github.com/neutron-org/neutron-query-relayer/internal/webserver"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/spf13/cobra"
)

var urlICQ string

const (
	UrlFlagName = "url"
	getTimeout  = time.Second * 5
)

// QueryCmd represents the query command
var QueryCmd = &cobra.Command{
	Use: "query",
}

func init() {
	QueryCmd.PersistentFlags().StringVarP(&urlICQ, UrlFlagName, "u", "http://localhost:10001", "server url")
	err := QueryCmd.MarkPersistentFlagRequired(UrlFlagName)
	if err != nil {
		log.Fatalf("could not initialize query command: %s", err)
	}

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

		response, err := get(url, webserver.UnsuccessfulTxsResource)
		if err != nil {
			return fmt.Errorf("failed to get unssuccesful txs: %w", err)
		}
		fmt.Printf("Unsuccessful txs:\n%s\n", response)

		return nil
	},
}

func get(host string, resource string) (string, error) {
	u, err := url.Parse(host)
	if err != nil {
		return "", fmt.Errorf("host parsing error: %w", err)
	}

	u.Path = resource

	client := http.Client{
		Timeout: getTimeout,
	}

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return "", fmt.Errorf("failed to build http request: %w", err)
	}

	res, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make http request: %w", err)
	}

	if res.StatusCode != 200 {
		return "", fmt.Errorf("got unexpected response status code: %d", res.StatusCode)
	}

	defer res.Body.Close()
	var body bytes.Buffer
	_, err = io.Copy(&body, res.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	return body.String(), nil
}
