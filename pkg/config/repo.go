package config

import (
	"driftive/pkg/config/repo"
	"driftive/pkg/utils"
	"fmt"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v2"
	"os"
)

var ErrMissingRepoConfig = fmt.Errorf("driftive.yml not found")

func loadRepoConfig(filePath string) (*repo.DriftiveRepoConfig, error) {
	log.Info().Msgf("Loading repo config from %s", filePath)
	fileContent, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	cfg := &repo.DriftiveRepoConfig{}
	err = yaml.Unmarshal(fileContent, cfg)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

func DetectRepoConfig(repoDir string) (*repo.DriftiveRepoConfig, error) {
	if os.Getenv("DRIFTIVE_REPO_CONFIG") != "" {
		envConfigStr := os.Getenv("DRIFTIVE_REPO_CONFIG")
		cfg := &repo.DriftiveRepoConfig{}
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

func DefaultRepoConfig() *repo.DriftiveRepoConfig {
	return &repo.DriftiveRepoConfig{
		AutoDiscover: repo.DriftiveRepoConfigAutoDiscover{
			Inclusions: []string{"**/terragrunt.hcl", "**/*.tf"},
			Exclusions: []string{".git/**", "**/modules/**", "**/.terragrunt-cache/**", "**/.terraform", "/terragrunt.hcl"},
			ProjectRules: []repo.AutoDiscoverRule{{
				Pattern:    "terragrunt.hcl",
				Executable: "terragrunt"}},
		},
	}
}
