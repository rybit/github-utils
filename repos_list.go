package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func listReposCmd() *cobra.Command {
	cmd := cobra.Command{
		Use: "list-repos",
		Run: func(cmd *cobra.Command, args []string) {
			listRepos()
		},
	}
	return &cmd
}

func listRepos() {
	readRepoPages(func(r repo) error {
		return enc(&r)
	})
}

func (r repo) Fields() []csvField {
	return []csvField{
		{"name", r.Name},
		{"private", r.Private},
		{"archived", r.Archived},
		{"disabled", r.Disabled},
		{"default_branch", r.DefaultBranch},
		{"url", r.HTMLURL},
	}
}

type repo struct {
	Name          string
	Archived      bool
	Private       bool
	PushedAt      time.Time `json:"pushed_at"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	DefaultBranch string    `json:"default_branch"`
	ID            int
	FullName      string `json:"full_name"`
	Owner         struct {
		Login     string
		ID        int
		Type      string
		SiteAdmin bool `json:"site_admin"`
	}
	HTMLURL         string `json:"html_url"`
	Description     string
	Fork            bool
	Size            int
	OpenIssuesCount int `json:"open_issues_count"`
	Disabled        bool
	Language        interface{} // idk what this is going to be
}

type repoPageIter func(r repo) error

func readRepoPages(iter repoPageIter) {
	page := 1
	reposProcessed := 0
	for {
		code, raw := queryGitHub(fmt.Sprintf("/orgs/netlify/repos?per_page=100&page=%d", page), "")
		if code != http.StatusOK {
			log.Info("Got a !200 response, assuming we got all the repos")
			return
		}

		repos := []repo{}
		log.Info("fetched a new page of repos", zap.Int("count", len(repos)), zap.Int("page", page))
		panicOnErr(json.Unmarshal(raw, &repos))
		for _, r := range repos {
			if skipArchive && r.Archived {
				log.Info("skipping archive repo",
					zap.String("repo", r.Name),
				)
				continue
			}
			if !strings.HasPrefix(r.Name, "netlify/") {
				r.Name = "netlify/" + r.Name
			}
			log.Info("starting to process repo",
				zap.String("repo", r.Name),
			)
			panicOnErr(iter(r))
			reposProcessed++
			if limit != 0 && reposProcessed >= limit {
				log.Info("Reached configured limit	")
				return
			}
		}
		if len(repos) == 0 {
			log.Info("No repos found, shutting down")
			return
		}
		page += 1
	}
}
