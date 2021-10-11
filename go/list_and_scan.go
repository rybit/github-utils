package main

import (
	"github.com/spf13/cobra"
)

func listAndScanCmd() *cobra.Command {
	var useCSV bool
	cmd := cobra.Command{
		Use: "list-and-scan",
		Run: func(cmd *cobra.Command, args []string) {
			if useCSV {
				enc = buildCSVEncoder(out, repoStatusFields)
			}
			searchReposAndScan()
		},
	}
	cmd.Flags().BoolVar(&useCSV, "csv", false, "if we should encode with csv")
	return &cmd
}

func searchReposAndScan() {
	readRepoPages(func(r repo) error {
		return enc(queryRepoForCI(r))
	})
}
