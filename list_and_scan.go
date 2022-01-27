package main

import (
	"github.com/spf13/cobra"
)

func listAndScanCmd() *cobra.Command {
	cmd := cobra.Command{
		Use: "list-and-scan",
		Run: func(cmd *cobra.Command, args []string) {
			searchReposAndScan()
		},
	}
	return &cmd
}

func searchReposAndScan() {
	readRepoPages(func(r repo) error {
		return enc(queryRepoForCI(r))
	})
}
