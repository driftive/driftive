package main

import (
	"context"
	"driftive/pkg/config"
	"driftive/pkg/config/discover"
	"driftive/pkg/config/repo"
	"driftive/pkg/drift"
	"driftive/pkg/git"
	"driftive/pkg/notification"
	"driftive/pkg/vcs"
	"driftive/pkg/vcs/vcstypes"
	"errors"
	"os"

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

func prepareStash(ctx context.Context, scmOps vcs.VCS, cfg *config.DriftiveConfig, repoConfig *repo.DriftiveRepoConfig) ([]*vcstypes.VCSIssue, []ChangedFile, []*vcstypes.VCSPullRequest) {
	var allOpenIssues []*vcstypes.VCSIssue
	changedFiles := make([]ChangedFile, 0)
	var allOpenPrs []*vcstypes.VCSPullRequest
	if cfg.GithubContext.IsValid() && cfg.GithubToken != "" {
		log.Info().Msg("Github context detected.")
		issues, err := scmOps.GetAllOpenRepoIssues(ctx)
		if err != nil {
			log.Fatal().Msgf("Failed to get open issues: %v", err)
		}
		allOpenIssues = issues

		allOpenPrs, err := scmOps.GetAllOpenPRs(ctx)
		if err != nil {
			log.Fatal().Msgf("Failed to get open pull requests: %v", err)
		}

		if repoConfig.Settings.SkipIfOpenPR {
			files, err := scmOps.GetChangedFilesForAllOpenPrs(ctx, allOpenPrs)
			if err != nil {
				log.Fatal().Msgf("Failed to get changed files for open PRs: %v", err)
			}
			changedFiles = files
		} else {
			log.Info().Msg("Not checking for changed files in open PRs because skip_if_open_pr is not enabled.")
		}
	}

	return allOpenIssues, changedFiles, allOpenPrs
}

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: ""})
	cfg := config.ParseConfig()
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

	if err != nil {
		log.Fatal().Msgf("Failed to create GitHub client: %v", err)
	}
	scmOps, err := vcs.NewVCS(cfg, repoConfig)
	if err != nil {
		log.Fatal().Msgf("Failed to create VCS client: %v", err)
	}

	openIssues, changedFiles, openPRs := prepareStash(ctx, scmOps, cfg, repoConfig)

	projects := discover.AutoDiscoverProjects(repoDir, repoConfig)
	log.Info().Msgf("Projects detected: %d", len(projects))
	driftDetector := drift.NewDriftDetector(repoDir, projects, cfg, repoConfig, openIssues, changedFiles, openPRs)
	analysisResult := driftDetector.DetectDrift(ctx)

	notification.NewNotificationHandler(cfg, repoConfig, scmOps).
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
	log.Info().Msgf("Options: concurrency: %d. github issues: %s. github pull requests: %s. slack %s. close resolved issues: %s. max opened issues: %d",
		cfg.Concurrency,
		parseOnOff(repoConfig.GitHub.Issues.Enabled),
		parseOnOff(repoConfig.GitHub.PullRequests.Enabled),
		parseOnOff(cfg.SlackWebhookUrl != ""),
		parseOnOff(repoConfig.GitHub.Issues.CloseResolved),
		repoConfig.GitHub.Issues.MaxOpenIssues)

	if repoConfig.GitHub.Issues.Enabled && (cfg.GithubToken == "" || cfg.GithubContext == nil || cfg.GithubContext.Repository == "" || cfg.GithubContext.RepositoryOwner == "") {
		log.Fatal().Msg("Github issues are enabled but the required Github token or context is not provided. " +
			"Use the --github-token flag or set the GITHUB_TOKEN environment variable. " +
			"Also, ensure that the GITHUB_CONTEXT environment variable is set in Github Actions.")
	}
}
