package config

import (
	"driftive/pkg/gh"
	"driftive/pkg/utils"
	"flag"
	"fmt"
	"os"
	"runtime/debug"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func validateArgs(repositoryUrl, repositoryPath, branch string) {
	if repositoryUrl == "" && repositoryPath == "" {
		usageError("either --repo-path or --repo-url is required")
	}
	if branch == "" && repositoryPath == "" {
		usageError("--branch is required when --repo-url is provided")
	}
}

func usageError(msg string) {
	fmt.Fprintf(os.Stderr, "driftive: %s\n\n", msg)
	flag.Usage()
	os.Exit(2)
}

func parseDriftiveToken() string {
	token := os.Getenv("DRIFTIVE_TOKEN")
	if token == "" {
		return ""
	}
	return token
}

// resolvedVersion returns the version to display. The compile-time value wins;
// otherwise we fall back to module info populated by `go install`.
func resolvedVersion(compileTime string) string {
	if compileTime != "" && compileTime != "dev" {
		return compileTime
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		if v := info.Main.Version; v != "" && v != "(devel)" {
			return v
		}
	}
	return "dev"
}

func setUsage() {
	flag.Usage = func() {
		out := flag.CommandLine.Output()
		fmt.Fprintln(out, "driftive — detect drift in Terraform, Terragrunt, and OpenTofu projects.")
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Usage:")
		fmt.Fprintln(out, "  driftive --repo-path <path> [flags]")
		fmt.Fprintln(out, "  driftive --repo-url <url> --branch <branch> [flags]")
		fmt.Fprintln(out, "  driftive --version")
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Flags:")
		flag.PrintDefaults()
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Environment variables:")
		fmt.Fprintln(out, "  DRIFTIVE_TOKEN   Bearer token for reporting results to Driftive Cloud.")
		fmt.Fprintln(out, "  GITHUB_CONTEXT   GitHub Actions context JSON (auto-set inside Actions).")
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Examples:")
		fmt.Fprintln(out, "  driftive --repo-path ./my-tf-repo")
		fmt.Fprintln(out, "  driftive --repo-url https://token@github.com/org/repo --branch main")
		fmt.Fprintln(out, "  driftive --version")
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Project discovery and notification routing are configured via driftive.yml")
		fmt.Fprintln(out, "at the repository root. See https://github.com/driftive/driftive for details.")
	}
}

func ParseConfig(version string) *DriftiveConfig {
	var repositoryUrl string
	var slackWebhookUrl string
	var branch string
	var repositoryPath string
	var concurrency int
	var logLevel string
	var enableStdoutResult bool
	var githubToken string
	var driftiveApiUrl string
	var exitCode bool
	var showVersion bool

	setUsage()

	flag.StringVar(&repositoryPath, "repo-path", "", "Path to the repository. If provided, the repository will not be cloned.")
	flag.StringVar(&repositoryUrl, "repo-url", "", "e.g. https://<token>@github.com/<org>/<repo>. If repo-path is provided, this is ignored.")
	flag.StringVar(&branch, "branch", "", "Repository branch")
	flag.StringVar(&slackWebhookUrl, "slack-url", "", "Slack webhook URL")
	flag.IntVar(&concurrency, "concurrency", 4, "Number of concurrent projects to check. Defaults to 4.")
	flag.StringVar(&logLevel, "log-level", "info", "Log level. Options: trace, debug, info, warn, error, fatal, panic")
	flag.BoolVar(&enableStdoutResult, "stdout", true, "Enable printing drift results to stdout")
	flag.StringVar(&githubToken, "github-token", "", "Github token")
	flag.BoolVar(&exitCode, "exit-code", false, "Exit with code 1 if any state drift is detected")
	flag.StringVar(&driftiveApiUrl, "api-url", "https://api.driftive.cloud", "Driftive API URL")
	flag.BoolVar(&showVersion, "version", false, "Print version information and exit")
	flag.BoolVar(&showVersion, "v", false, "Shorthand for --version")
	flag.Parse()

	if showVersion {
		fmt.Fprintf(os.Stdout, "driftive %s\n", resolvedVersion(version))
		os.Exit(0)
	}

	validateArgs(repositoryUrl, repositoryPath, branch)

	zerolog.SetGlobalLevel(utils.ParseLogLevel(logLevel))

	ghContext, err := gh.ParseGHActionContextEnvVar()
	if err != nil {
		log.Warn().Msgf("Failed to parse github action context. %v", err)
	}
	if ghContext != nil {
		err := ghContext.ValidateGithubContext()
		if err != nil {
			log.Fatal().Msgf("Invalid github context. %v", err)
		}
	}

	driftiveToken := parseDriftiveToken()

	return &DriftiveConfig{
		RepositoryUrl:      repositoryUrl,
		Branch:             branch,
		RepositoryPath:     strings.TrimSuffix(repositoryPath, utils.PathSeparator),
		Concurrency:        concurrency,
		LogLevel:           logLevel,
		EnableStdoutResult: enableStdoutResult,
		SlackWebhookUrl:    slackWebhookUrl,
		GithubToken:        githubToken,
		GithubContext:      ghContext,
		ExitCode:           exitCode,
		DriftiveApiUrl:     driftiveApiUrl,
		DriftiveToken:      driftiveToken,
	}
}
