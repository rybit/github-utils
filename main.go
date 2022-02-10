package main

import (
	"io"
	"os"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var ghToken string
var log *zap.Logger
var skipArchive, verbose, pretty bool
var limit int
var enc encoder
var out io.WriteCloser = os.Stdout

var ghQueries int

func main() {
	var err error
	cfg := zap.NewDevelopmentConfig()
	cfg.Level.SetLevel(zap.InfoLevel)
	cfg.DisableCaller = true
	log, err = cfg.Build()
	panicOnErr(err)

	root := cobra.Command{}
	root.PersistentFlags().StringP("token", "t", "", "the github access token")
	root.PersistentFlags().BoolVar(&skipArchive, "skip-archived", false, "if we should skip archived repos")
	root.PersistentFlags().BoolVar(&verbose, "verbose", false, "if we log the paths we query")
	root.PersistentFlags().BoolVar(&pretty, "pretty", false, "if the json should be pretty")
	root.PersistentFlags().IntVar(&limit, "limit", 0, "a limit on the number of repos to scan")
	root.PersistentFlags().String("out", "", "an optional file to append to, default is stdout")

	cmds := setPreActions(
		queryGitHubCmd(),

		projectCmd(),

		supportCSV(ciScanCmd()),
		supportCSV(listReposCmd()),
		supportCSV(listAndScanCmd()),
		supportCSV(listGoMods()),
	)
	root.AddCommand(cmds...)

	panicOnErr(root.Execute())
}

func setPreActions(commands ...*cobra.Command) []*cobra.Command {
	prerun := func(cmd *cobra.Command, args []string) {
		setVerbosity(cmd)
		setToken(cmd)
		setOutput(cmd)
	}

	postrun := func(cmd *cobra.Command, args []string) {
		panicOnErr(out.Close())
		log.Sugar().Debugf("did %d queries to github", ghQueries)
	}

	for _, c := range commands {
		if c.Run != nil {
			c.PreRun = prerun
			c.PostRun = postrun
		}
		if c.HasSubCommands() {
			setPreActions(c.Commands()...)
		}
	}
	return commands
}

func setToken(cmd *cobra.Command) {
	ghToken = os.Getenv("GITHUB_ACCESS_TOKEN")
	if cliToken, _ := cmd.Flags().GetString("token"); cliToken != "" {
		ghToken = cliToken
	}
	if ghToken == "" {
		panic("must provide the token via env var GITHUB_ACCESS_TOKEN or the flag")
	}
}

func setOutput(cmd *cobra.Command) {
	outName, _ := cmd.Flags().GetString("out")
	if outName != "" {
		f, err := os.OpenFile(outName, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
		panicOnErr(err)
		out = f
	}
	enc = buildJSONEncoder(out)
}

func supportCSV(cmd *cobra.Command) *cobra.Command {
	var useCSV bool
	cmd.Flags().BoolVar(&useCSV, "csv", false, "if we should encode with csv")
	setup := cmd.PreRun
	cmd.PreRun = func(cmd *cobra.Command, args []string) {
		setup(cmd, args)
		if useCSV {
			enc = buildCSVEncoder(out)
		}
	}
	return cmd
}

func setVerbosity(cmd *cobra.Command) {
	if verbose {
		cfg := zap.NewDevelopmentConfig()
		cfg.Level.SetLevel(zap.DebugLevel)
		cfg.DisableCaller = true
		dlog, err := cfg.Build()
		panicOnErr(err)
		log = dlog
	}
}
