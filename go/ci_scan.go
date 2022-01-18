package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func ciScanCmd() *cobra.Command {
	var file string
	cmd := cobra.Command{
		Use: "scan-ci [repo]",
		Run: func(cmd *cobra.Command, args []string) {
			walkReposForCI(append(args, loadRepos(file)...))
		},
	}
	cmd.Flags().StringVar(&file, "file", "", "a file of new line delimited repos to get")
	return &cmd
}

func walkReposForCI(repos []string) {
	for i, r := range repos {
		log.Info("starting query for repo's state",
			zap.String("repo", r),
			zap.Int("index", i),
			zap.Int("total", len(repos)),
		)
		state := queryRepoForCI(repo{Name: r})
		panicOnErr(enc(state))
	}
}

func queryRepoForCI(repo repo) repoStatus {
	state := repoStatus{
		repo:        repo,
		Jenkinsfile: queryForFile(repo.Name, "Jenkinsfile"),
		CircleCI:    queryForFile(repo.Name, ".circleci/config.yml"),
		RootTOML:    queryForFile(repo.Name, "netlify.toml"),
		Security:    queryForFile(repo.Name, ".github/SECURITY.MD"),
	}

	if entry := queryForFileContent(repo.Name, ".github/CODEOWNERS"); entry != nil {
		state.CodeOwners = cleanCodeowners(*entry)
	}

	if code, raw := queryGitHub(fmt.Sprintf("repos/%s/contents/.github/workflows", repo.Name), ""); code == http.StatusOK {
		ghaFiles := make([]fileEntry, 0)
		panicOnErr(json.Unmarshal(raw, &ghaFiles))
		for _, file := range ghaFiles {
			switch file.Name {
			case "fossa.yml":
				state.Fossa = true
			case "stalebot.yml":
				state.Stalebot = true
			case "renovate.yml":
				state.Renovate = true
			default:
				state.Actions = append(state.Actions, file.Name)
			}
		}
	}
	return state
}

func loadRepos(file string) []string {
	if file == "" {
		return nil
	}
	repos := []string{}
	handle, err := os.Open(file)
	panicOnErr(err)
	scan := bufio.NewScanner(handle)
	for scan.Scan() {
		v := strings.TrimSpace(scan.Text())
		if v != "" {
			repos = append(repos, v)
		}
	}

	return repos
}

type repoStatus struct {
	repo
	Security    bool
	Jenkinsfile bool
	CircleCI    bool
	CodeOwners  []string
	Stalebot    bool
	Fossa       bool
	Renovate    bool
	RootTOML    bool
	Actions     []string
}

func cleanCodeowners(e fileEntry) []string {
	owners := map[string]struct{}{}
	curOwner := []byte{}

	for _, b := range e.Contents() {
		if b == '@' {
			curOwner = append(curOwner, b)
			continue
		}
		if len(curOwner) > 0 {
			curOwner = append(curOwner, b)
		}
		if strings.TrimSpace(string(b)) == "" && len(curOwner) > 0 {

			for _, o := range strings.Split(strings.TrimPrefix(string(curOwner), "@netlify/"), ",") {
				owners[strings.TrimSpace(o)] = struct{}{}
			}
			curOwner = []byte{}
		}
	}
	out := []string{}
	for o := range owners {
		out = append(out, o)
	}
	return out
}
