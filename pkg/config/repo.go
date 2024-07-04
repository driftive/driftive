package config

import (
	"driftive/pkg/config/repo"
	"gopkg.in/yaml.v2"
	"os"
)

func DetectRepoConfig(repoDir string) (*repo.DriftiveRepoConfig, error) {
	if os.Getenv("DRIFTIVE_REPO_CONFIG") != "" {
		envConfigStr := os.Getenv("DRIFTIVE_REPO_CONFIG")
		cfg := &repo.DriftiveRepoConfig{}
		err := yaml.Unmarshal([]byte(envConfigStr), cfg)
		if err != nil {
			return nil, err
		}
	}

	if _, err := os.Stat(repoDir + "/.driftive.yml"); err == nil {
		fileContent, err := os.ReadFile(repoDir + "/.driftive.yml")
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
	return nil, nil
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
