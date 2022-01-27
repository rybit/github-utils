package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func queryGitHubCmd() *cobra.Command {
	cmd := cobra.Command{
		Use:  "query-github",
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			path := args[0]
			status, raw := queryGitHub(path, "")
			log.Info("finished querying github", zap.Int("status", status))
			fmt.Println(string(raw))
		},
	}
	return &cmd
}
