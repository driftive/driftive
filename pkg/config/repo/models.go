package repo

type AutoDiscoverRule struct {
	Pattern    string `json:"pattern" yaml:"pattern"`
	Executable string `json:"executable" yaml:"executable" validate:"omitempty,oneof=terraform tofu terragrunt"`
}

type DriftiveRepoConfigGitHubIssues struct {
	// Enabled is used to enable or disable GitHub issues integration
	Enabled bool `json:"enabled" yaml:"enabled"`
	// CloseResolved is used to close resolved driftive issues
	CloseResolved bool `json:"close_resolved" yaml:"close_resolved"`
	// MaxOpenIssues is the maximum number of open issues to have at any time
	MaxOpenIssues int `json:"max_open_issues" yaml:"max_open_issues"`
}

type DriftiveRepoConfigGitHub struct {
	Issues DriftiveRepoConfigGitHubIssues `json:"issues" yaml:"issues"`
}

// DriftiveRepoConfig is used to configure driftive for a repository.
// It may be defined in a .driftive.yaml file in the repository or passed via environment variable.
type DriftiveRepoConfig struct {
	AutoDiscover DriftiveRepoConfigAutoDiscover `json:"auto_discover" yaml:"auto_discover"`
	GitHub       DriftiveRepoConfigGitHub       `json:"github" yaml:"github"`
}

// DriftiveRepoConfigAutoDiscover is used to configure auto discovery of projects in a repository
type DriftiveRepoConfigAutoDiscover struct {
	// Enabled is used to enable or disable auto discovery
	Enabled bool `json:"enabled" yaml:"enabled"`
	// Inclusions list of glob patterns to include in auto discovery
	Inclusions []string `json:"inclusions" yaml:"inclusions"`
	// Exclusions list of glob patterns to exclude in auto discovery
	Exclusions []string `json:"exclusions" yaml:"exclusions"`
	// ProjectRules list of rules to apply to auto discovered projects
	ProjectRules []AutoDiscoverRule `json:"project_rules" yaml:"project_rules"`
}
