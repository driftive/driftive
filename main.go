package main

import (
	"context"
	"driftive/pkg/config"
	"driftive/pkg/config/discover"
	"driftive/pkg/config/repo"
	"driftive/pkg/drift"
	"driftive/pkg/git"
	"driftive/pkg/notification"
	"driftive/pkg/notification/driftive"
	"driftive/pkg/vcs"
	"driftive/pkg/vcs/vcstypes"
	"errors"
	"os"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// determineRepositoryDir returns the repository path to use. If repositoryPath is provided, it is returned. Otherwise, the repositoryUrl is returned.
// The second return value is true if the repositoryPath should be deleted after the program finishes.
func determineRepositoryDir(ctx context.Context, repositoryUrl, repositoryPath, branch string) (string, bool) {
	if repositoryPath != "" {
		return repositoryPath, false
	}

	createdDir, err := os.MkdirTemp("", "driftive")
	if err != nil {
		panic(err)
	}

	log.Debug().Msgf("Created temp dir: %s", createdDir)
	err = git.CloneRepo(ctx, repositoryUrl, branch, createdDir)
	if err != nil {
		panic(err)
	}
	log.Info().Msgf("Cloned repo: %s to %s", repositoryUrl, createdDir)

	return createdDir, true
}

type ChangedFile = string

func prepareStash(ctx context.Context, scmOps vcs.VCS, cfg *config.DriftiveConfig, repoConfig *repo.DriftiveRepoConfig) ([]*vcstypes.VCSIssue, []ChangedFile) {
	var allOpenIssues []*vcstypes.VCSIssue
	changedFiles := make([]ChangedFile, 0)
	if cfg.GithubContext.IsValid() && cfg.GithubToken != "" {
		log.Info().Msg("Github context detected.")
		issues, err := scmOps.GetAllOpenRepoIssues(ctx)
		if err != nil {
			log.Fatal().Msgf("Failed to get open issues: %v", err)
		}
		allOpenIssues = issues

		if repoConfig.Settings.SkipIfOpenPR {
			files, err := scmOps.GetChangedFilesForAllPRs(ctx)
			if err != nil {
				log.Fatal().Msgf("Failed to get changed files for open PRs: %v", err)
			}
			changedFiles = files
		} else {
			log.Info().Msg("Not checking for changed files in open PRs because skip_if_open_pr is not enabled.")
		}
	}

	return allOpenIssues, changedFiles
}

// version is overridden at release time via -ldflags "-X main.version=...".
var version = "dev"

// startLiveReporter wires live progress reporting onto the detector, under the same gate as the
// terminal upload. Returns nil when the Driftive API is not configured, in which case the scan
// makes no network calls and behaves exactly as before.
func startLiveReporter(ctx context.Context, cfg *config.DriftiveConfig, detector *drift.DriftDetector, runKey string, totalProjects int) *driftive.LiveReporter {
	if !cfg.DriftiveAPIEnabled() {
		return nil
	}

	reporter := driftive.NewLiveReporter(cfg.DriftiveApiUrl, cfg.DriftiveToken, runKey, totalProjects)
	detector.OnProjectStart = reporter.ProjectStarted
	detector.OnProjectDone = reporter.ProjectFinished
	reporter.Start(ctx)
	log.Info().Msg("Live progress reporting enabled.")
	return reporter
}

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: ""})
	cfg := config.ParseConfig(version)
	ctx := context.Background()

	repoDir, shouldDelete := determineRepositoryDir(ctx, cfg.RepositoryUrl, cfg.RepositoryPath, cfg.Branch)
	if shouldDelete {
		log.Debug().Msg("Temp dir will be deleted after driftive finishes.")
		defer os.RemoveAll(repoDir)
	}

	repoConfig, err := repo.DetectRepoConfig(repoDir)
	if err != nil && !errors.Is(err, repo.ErrMissingRepoConfig) {
		log.Fatal().Msgf("Failed to load repository config. %v", err)
	}
	repoConfig = repo.RepoConfigOrDefault(repoConfig)
	repo.ValidateRepoConfig(repoConfig)
	showInitMessage(cfg, repoConfig)

	scmOps, err := vcs.NewVCS(cfg, repoConfig)
	if err != nil {
		log.Fatal().Msgf("Failed to create VCS client: %v", err)
	}

	openIssues, changedFiles := prepareStash(ctx, scmOps, cfg, repoConfig)

	projects := discover.AutoDiscoverProjects(repoDir, repoConfig)
	log.Info().Msgf("Projects detected: %d", len(projects))
	driftDetector := drift.NewDriftDetector(repoDir, projects, cfg, repoConfig, openIssues, changedFiles)

	// One key identifies this run to the API across every progress post and the terminal upload.
	runKey := uuid.NewString()
	liveReporter := startLiveReporter(ctx, cfg, &driftDetector, runKey, len(projects))

	analysisResult := driftDetector.DetectDrift(ctx)

	// Stopped before the terminal upload so a progress post cannot race the finalize.
	if liveReporter != nil {
		liveReporter.Stop()
	}

	notification.NewNotificationHandler(cfg, repoConfig, scmOps, runKey).
		HandleNotifications(ctx, analysisResult)

	if analysisResult.TotalDrifted <= 0 {
		log.Info().Msg("No drifts detected")
	} else if cfg.ExitCode {
		os.Exit(1)
	}
}

func parseOnOff(enabled bool) string {
	if enabled {
		return "on"
	}
	return "off"
}

func showInitMessage(cfg *config.DriftiveConfig, repoConfig *repo.DriftiveRepoConfig) {
	log.Info().Msg("Starting driftive...")
	log.Info().Msgf("Options: concurrency: %d. github issues: %s. slack: %s. close resolved issues: %s. max opened issues: %d",
		cfg.Concurrency,
		parseOnOff(repoConfig.GitHub.Issues.Enabled),
		parseOnOff(cfg.SlackWebhookUrl != ""),
		parseOnOff(repoConfig.GitHub.Issues.CloseResolved),
		repoConfig.GitHub.Issues.MaxOpenIssues)

	if repoConfig.GitHub.Issues.Enabled && (cfg.GithubToken == "" || cfg.GithubContext == nil || cfg.GithubContext.Repository == "" || cfg.GithubContext.RepositoryOwner == "") {
		log.Fatal().Msg("Github issues are enabled but the required Github token or context is not provided. " +
			"Use the --github-token flag or set the GITHUB_TOKEN environment variable. " +
			"Also, ensure that the GITHUB_CONTEXT environment variable is set in Github Actions.")
	}
}
