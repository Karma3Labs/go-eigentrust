package cmd

import (
	"strings"

	"github.com/spf13/cobra"
)

const (
	localEndpoint = "http://localhost:8080/basic/v1"
)

var (
	// basicCmd represents the basic command
	basicCmd = &cobra.Command{
		Use:   "basic",
		Short: "EigenTrust Basic API Client",
		Long:  `EigenTrust Basic API Client`,
	}
	endpoint         string
	useLocalEndpoint bool
)

func init() {
	rootCmd.AddCommand(basicCmd)
	basicCmd.PersistentFlags().StringVarP(&endpoint, "endpoint", "H",
		"https://api.k3l.io/basic/v1",
		`API endpoint address`)
	basicCmd.PersistentFlags().BoolVarP(&useLocalEndpoint, "local", "L",
		false,
		`use local API endpoint at http://localhost:8080 (ignores --endpoint)`)
}

func basicSetupEndpoint() {
	if useLocalEndpoint {
		endpoint = localEndpoint
		logger.Info().Str("endpoint", endpoint).Msg("using local endpoint")
	}
	if !strings.HasSuffix(endpoint, "/") {
		endpoint += "/"
	}
}
