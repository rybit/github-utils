package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func listProjectsCmd() *cobra.Command {
	cmd := cobra.Command{
		Use: "list-projects",
		Run: func(cmd *cobra.Command, args []string) {
			listProjects()
		},
	}
	return &cmd
}

func listProjects() {
	readProjectPages(func(p project) error {
		return enc(&p)
	})
}

type project struct {
	ID         string
	Name       string
	Body       string
	State      string
	ColumnsURL string `json:"columns_url"`
}

func readProjectPages(iter func(p project) error) {
	page := 1
	projectsProcessed := 0
	for {
		code, raw := queryGitHub(fmt.Sprintf("/orgs/netlify/projects?per_page=100&page=%d", page), "")
		if code != http.StatusOK {
			log.Info("Got a !200 response, assuming we got all the repos")
			return
		}

		objs := []project{}
		log.Info("fetched a new page", zap.Int("count", len(objs)), zap.Int("page", page))
		panicOnErr(json.Unmarshal(raw, &objs))
		for _, r := range objs {
			if skipArchive && r.State != "open" {
				log.Info("skipping closed project",
					zap.String("project", r.Name),
				)
				continue
			}

			log.Info("starting to process project",
				zap.String("project", r.Name),
			)
			panicOnErr(iter(r))
			projectsProcessed++
			if limit != 0 && projectsProcessed >= limit {
				log.Info("Reached configured limit	")
				return
			}
		}
		if len(objs) == 0 {
			log.Info("No repos found, shutting down")
			return
		}
		page += 1
	}
}
