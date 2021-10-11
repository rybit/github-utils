package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

type encoder func(obj interface{}) error
type fieldExtractor func(obj interface{}) interface{}

func buildJSONEncoder(out io.WriteCloser) encoder {
	writer := json.NewEncoder(out)
	return func(obj interface{}) error {
		return writer.Encode(&obj)
	}
}

func buildCSVEncoder(out io.WriteCloser, fields []csvWriter) encoder {
	writer := csv.NewWriter(out)
	var header []string
	for _, f := range fields {
		header = append(header, f.title)
	}
	panicOnErr(writer.Write(header))

	return func(obj interface{}) error {
		var entries []string
		for _, f := range fields {
			entries = append(entries, fmt.Sprintf("%v", f.getter(obj)))
		}
		panicOnErr(writer.Write(entries))

		writer.Flush()
		return writer.Error()
	}
}

var goModFields = []csvWriter{
	{"name", wrapGoModField(func(r goModRef) interface{} { return r.Repo })},
	{"version", wrapGoModField(func(r goModRef) interface{} { return r.Version })},
	{"private", wrapGoModField(func(r goModRef) interface{} { return r.Private })},
}

type csvWriter struct {
	title  string
	getter fieldExtractor
}

var repoStatusFields = []csvWriter{
	{"name", wrapRepoField(func(s repoStatus) interface{} { return s.Name })},
	{"archived", wrapRepoField(func(s repoStatus) interface{} { return s.Archived })},
	{"code owners", wrapRepoField(func(s repoStatus) interface{} { return strings.Join(s.CodeOwners, ",") })},
	{"jenkinsfile", wrapRepoField(func(s repoStatus) interface{} { return s.Jenkinsfile })},
	{"circle ci", wrapRepoField(func(s repoStatus) interface{} { return s.CircleCI })},
	{"default branch", wrapRepoField(func(s repoStatus) interface{} { return s.DefaultBranch })},
	{"last push", wrapRepoField(func(s repoStatus) interface{} { return s.PushedAt })},
	{"private", wrapRepoField(func(s repoStatus) interface{} { return s.Private })},
	{"fossa", wrapRepoField(func(s repoStatus) interface{} { return s.Fossa })},
	{"renovate", wrapRepoField(func(s repoStatus) interface{} { return s.Renovate })},
	{"stalebot", wrapRepoField(func(s repoStatus) interface{} { return s.Stalebot })},
	{"netlify.toml", wrapRepoField(func(s repoStatus) interface{} { return s.RootTOML })},
	{"security", wrapRepoField(func(s repoStatus) interface{} { return s.Security })},
	{"github actions", wrapRepoField(func(s repoStatus) interface{} { return strings.Join(s.Actions, ",") })},
}

func wrapGoModField(f func(r goModRef) interface{}) fieldExtractor {
	return func(obj interface{}) interface{} { return f(obj.(goModRef)) }
}

func wrapRepoField(f func(r repoStatus) interface{}) fieldExtractor {
	return func(obj interface{}) interface{} { return f(obj.(repoStatus)) }
}
