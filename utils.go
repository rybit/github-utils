package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
)

func queryForFile(repo, path string) bool {
	code, _ := queryGitHub(fmt.Sprintf("repos/%s/contents/%s", repo, path), "")
	return code/100 == 2
}
func queryForFileContent(repo, path string) *fileEntry {
	code, raw := queryGitHub(fmt.Sprintf("repos/%s/contents/%s", repo, path), "")
	if code == http.StatusOK {
		var e fileEntry
		panicOnErr(json.Unmarshal(raw, &e))
		return &e
	}
	return nil
}

type fileEntry struct {
	Name        string
	Path        string
	URL         string `json:"html_url"`
	RawContent  string `json:"content"`
	DownloadURL string `json:"download_url"`
	Encoding    string
	Type        string
}

func (e *fileEntry) Contents() []byte {
	if e.Type == "file" && e.RawContent != "" {
		switch e.Encoding {
		case "base64":
			res, err := base64.StdEncoding.DecodeString(e.RawContent)
			panicOnErr(err)
			return res
		}
		panic(fmt.Sprintf("unexpected encoding: %s", e.Encoding))
	}

	return nil
}
