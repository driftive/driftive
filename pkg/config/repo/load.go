package repo

import (
	"driftive/pkg/utils"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
	"os"
)

func loadRepoConfig(filePath string) (*DriftiveRepoConfig, error) {
	log.Info().Msgf("Loading repo config from %s", filePath)
	fileContent, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	cfg := &DriftiveRepoConfig{
		Settings: DriftiveRepoConfigSettings{
			SkipIfOpenPR: true,
		},
	}
	err = yaml.Unmarshal(fileContent, cfg)
	if err != nil {
		return nil, err
	}

	if cfg.GitHub.Issues.MaxOpenIssues == 0 {
		cfg.GitHub.Issues.MaxOpenIssues = 10
	}

	if cfg.GitHub.Issues.Errors.MaxOpenIssues == 0 {
		cfg.GitHub.Issues.Errors.MaxOpenIssues = 5
	}

	if cfg.GitHub.Summary.IssueTitle == "" {
		cfg.GitHub.Summary.IssueTitle = "Driftive Summary"
	}

	return cfg, nil
}

func DetectRepoConfig(repoDir string) (*DriftiveRepoConfig, error) {
	if os.Getenv("DRIFTIVE_REPO_CONFIG") != "" {
		envConfigStr := os.Getenv("DRIFTIVE_REPO_CONFIG")
		cfg := &DriftiveRepoConfig{}
		err := yaml.Unmarshal([]byte(envConfigStr), cfg)
		if err != nil {
			return nil, err
		}
	}

	if _, err := os.Stat(repoDir + utils.PathSeparator + "driftive.yml"); err == nil {
		return loadRepoConfig(repoDir + utils.PathSeparator + "driftive.yml")
	}
	if _, err := os.Stat(repoDir + utils.PathSeparator + "driftive.yaml"); err == nil {
		return loadRepoConfig(repoDir + utils.PathSeparator + "driftive.yaml")
	}
	return nil, ErrMissingRepoConfig
}
