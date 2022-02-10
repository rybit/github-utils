package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/spf13/cobra"
)

func transferRepoCmd() *cobra.Command {
	var teamIDs []int
	cmd := cobra.Command{
		Use: "transfer-repo <repo> <org>",
		Run: func(cmd *cobra.Command, args []string) {
			transferRepo(args[0], args[1], teamIDs)
		},
		Args: cobra.ExactArgs(2),
	}
	cmd.Flags().IntSliceVar(&teamIDs, "team", teamIDs, "a team ID to associate")

	return &cmd
}

func transferRepo(repoName, destOrg string, teams []int) {
	if !strings.HasPrefix(repoName, "netlify") {
		repoName = "netlify/" + repoName
	}
	body := struct {
		NewOwner string `json:"new_owner"`
		TeamIDs  []int  `json:"team_ids,omitempty"`
	}{
		NewOwner: destOrg,
		TeamIDs:  teams,
	}

	payload, err := json.Marshal(&body)
	panicOnErr(err)

	code, raw := queryGitHub(fmt.Sprintf("/repos/%s/transfer", repoName), withPayload(payload), withMethod(http.MethodPost))
	requireCode(code, http.StatusAccepted, raw)

	parts := strings.SplitAfterN(repoName, "/", 2)
	fmt.Printf("moved repo to https://github.com/%s/%s\n", destOrg, parts[len(parts)-1])
}
