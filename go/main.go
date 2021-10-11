package main

import (
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var ghToken string
var log *zap.Logger
var skipArchive, verbose bool
var limit int
var enc encoder
var out io.WriteCloser = os.Stdout

func main() {
	var err error
	log, err = zap.NewDevelopment()
	panicOnErr(err)
	root := cobra.Command{}
	root.PersistentFlags().StringP("token", "t", "", "the github access token")
	root.PersistentFlags().BoolVar(&skipArchive, "skip-archived", false, "if we should skip archived repos")
	root.PersistentFlags().BoolVar(&verbose, "verbose", false, "if we log the paths we query")
	root.PersistentFlags().IntVar(&limit, "limit", 0, "a limit on the number of repos to scan")
	root.PersistentFlags().String("out", "", "an optional file to append to, default is stdout")

	root.AddCommand(setPreActions(ciScanCmd(), listReposCmd(), listAndScanCmd(), listGoMods(), queryGitHubCmd())...)
	panicOnErr(root.Execute())
}

func setPreActions(commands ...*cobra.Command) []*cobra.Command {
	for _, c := range commands {
		c.PreRun = func(cmd *cobra.Command, args []string) {
			ghToken = os.Getenv("GITHUB_ACCESS_TOKEN")
			if cliToken, _ := cmd.Flags().GetString("token"); cliToken != "" {
				ghToken = cliToken
			}
			if ghToken == "" {
				panic("must provide the token via env var GITHUB_ACCESS_TOKEN or the flag")
			}
			outName, _ := cmd.Flags().GetString("out")
			if outName != "" {
				f, err := os.OpenFile(outName, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
				panicOnErr(err)
				out = f
			}
			enc = buildJSONEncoder(out)
		}
		c.PostRun = func(cmd *cobra.Command, args []string) {
			panicOnErr(out.Close())
		}
	}
	return commands
}

func panicOnErr(err error) {
	if err != nil {
		panic(err)
	}
}

func queryGitHub(path, accept string) (int, []byte) {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	url := "https://api.github.com" + path
	if verbose {
		log.Info("querying github", zap.String("url", url))
	}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	panicOnErr(err)
	if accept == "" {
		// accept = "application/vnd.github.inertia-preview+json"
		accept = "application/vnd.github.v3+json"
	}
	req.Header.Set("Accept", accept)
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
			return queryGitHub(path, accept)
		}
	}

	defer rsp.Body.Close()
	res, err := io.ReadAll(rsp.Body)
	panicOnErr(err)
	return rsp.StatusCode, res
}
