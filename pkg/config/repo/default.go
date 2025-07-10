package repo

func DefaultRepoConfig() *DriftiveRepoConfig {
	return &DriftiveRepoConfig{
		GitHub: DriftiveRepoConfigGitHub{
			Issues: DriftiveRepoConfigGitHubIssues{
				Enabled:       false,
				CloseResolved: false,
				MaxOpenIssues: 10,
			},
			PullRequests: DriftiveRepoConfigGithubPullRequests{
				Enabled:             false,
				CloseResolved:       false,
				MaxOpenPullRequests: 10,
			},
		},
		AutoDiscover: DriftiveRepoConfigAutoDiscover{
			Inclusions: []string{"**/terragrunt.hcl", "**/*.tf"},
			Exclusions: []string{".git/**", "**/modules/**", "**/.terragrunt-cache/**", "**/.terraform", "/terragrunt.hcl"},
			ProjectRules: []AutoDiscoverRule{
				{
					Pattern:    "terragrunt.hcl",
					Executable: "terragrunt"},
				{
					Pattern:    "*.tf",
					Executable: "terraform",
				}},
		},
		Settings: DriftiveRepoConfigSettings{
			SkipIfOpenPR: true,
		},
	}
}
