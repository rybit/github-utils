package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
)

func queryForFile(repo, path string) bool {
	code, _ := queryGitHub(fmt.Sprintf("repos/%s/contents/%s", repo, path))
	return code/100 == 2
}

func queryForFileContent(repo, path string) *fileEntry {
	code, raw := queryGitHub(fmt.Sprintf("repos/%s/contents/%s", repo, path))
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

func queryByPage(path string, cb func(raw []byte) bool) {
	page := 1
	for {
		code, raw := queryGitHub(fmt.Sprintf("%s?per_page=100&page=%d", path, page))
		if code != http.StatusOK {
			log.Info("Got a !200 response, assuming we got all the pages")
			return
		}

		log.Debug("fetched a new page", zap.Int("page", page))
		if !cb(raw) {
			log.Debug("Finished scrolling pages")
			return
		}
		page++
	}
}

func panicOnErr(err error) {
	if err != nil {
		panic(err)
	}
}

type opt func(r *http.Request)

func withAccept(accept string) opt {
	return func(r *http.Request) {
		r.Header.Set("Accept", accept)
	}
}

func withPayload(data []byte) opt {
	return func(r *http.Request) {
		r.Body = io.NopCloser(bytes.NewReader(data))
		log.Sugar().Debugf("query payload: %s", string(data))
	}
}

func withMethod(method string) opt {
	return func(r *http.Request) {
		r.Method = method
	}
}

func queryGitHub(path string, opts ...opt) (int, []byte) {
	ghQueries++

	prefix := "https://api.github.com"
	if !strings.HasPrefix(path, prefix) {
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}

		path = prefix + path
	}

	req, err := http.NewRequest(http.MethodGet, path, nil)
	panicOnErr(err)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	for _, o := range opts {
		o(req)
	}
	log.Debug("querying github",
		zap.String("url", req.URL.String()),
		zap.String("method", req.Method),
		zap.Bool("has_body", req.Body != nil),
	)

	req.SetBasicAuth("rybit", ghToken)
	rsp, err := http.DefaultClient.Do(req)
	panicOnErr(err)

	if remaining := rsp.Header.Get("x-ratelimit-remaining"); remaining != "" {
		left, err := strconv.Atoi(remaining)
		panicOnErr(err)
		if left == 0 {
			epoch, err := strconv.Atoi(rsp.Header.Get("x-ratelimit-reset"))
			panicOnErr(err)
			ts := time.Unix(int64(epoch), 0)
			log.Warn("Rate limit exceeded - going to wait for it.",
				zap.Time("resume", ts),
				zap.Duration("wait", ts.Sub(time.Now())),
			)
			tick := time.NewTicker(time.Minute)
			for {
				<-tick.C
				log.Info("Still waiting for the right time",
					zap.Time("resume", ts),
					zap.Duration("wait", ts.Sub(time.Now())),
				)

				if time.Now().After(ts) {
					break
				}
			}
			log.Info("Resuming, making that github query now")
			return queryGitHub(path, opts...)
		}
	}

	defer rsp.Body.Close()
	res, err := io.ReadAll(rsp.Body)
	panicOnErr(err)
	return rsp.StatusCode, res
}

func requireCode(expected, actual int, payload []byte) {
	if expected != actual {
		panic(fmt.Sprintf("unexpected response code: %d: %s", actual, string(payload)))
	}
}
