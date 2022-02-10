package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func queryGitHubCmd() *cobra.Command {
	var acceptRaw bool
	cmd := cobra.Command{
		Use:  "query-github",
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			path := args[0]
			var opts []opt
			if acceptRaw {
				opts = []opt{withAccept("application/vnd.github.v3.raw")}
			}
			status, raw := queryGitHub(path, opts...)
			log.Info("finished querying github", zap.Int("status", status))
			fmt.Println(string(raw))
		},
	}
	cmd.Flags().BoolVar(&acceptRaw, "raw", false, "if we should use the raw accept header")

	return &cmd
}
