package main

import (
	"bufio"
	"bytes"
	"strings"

	"github.com/spf13/cobra"
)

func listGoMods() *cobra.Command {
	cmd := cobra.Command{
		Use: "list-go-mods",
		Run: func(cmd *cobra.Command, args []string) {
			searchReposForGoMod()
		},
	}
	return &cmd
}

type goModRef struct {
	Repo    string
	Version string
	Private bool
}

func (r goModRef) Fields() []csvField {
	return []csvField{
		{"name", r.Repo},
		{"version", r.Version},
		{"private", r.Private},
	}
}

func searchReposForGoMod() {
	readRepoPages(func(r repo) error {
		if entry := queryForFileContent(r.Name, "go.mod"); entry != nil {
			scan := bufio.NewScanner(bytes.NewReader(entry.Contents()))

			for scan.Scan() {
				txt := scan.Text()
				if strings.Contains(txt, "github.com/netlify/netlify-commons") {
					fields := strings.Fields(txt)
					if len(fields) > 1 {
						return enc(goModRef{
							Repo:    r.Name,
							Private: r.Private,
							Version: fields[1],
						})
					}
				}
			}
			return nil
		}
		log.Info("no go.mod found")
		return nil
	})
}
