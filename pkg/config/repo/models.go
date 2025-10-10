package repo

type AutoDiscoverRule struct {
	Pattern    string `json:"pattern" yaml:"pattern"`
	Executable string `json:"executable" yaml:"executable" validate:"omitempty,oneof=terraform tofu terragrunt"`
}

type DriftiveRepoConfigGitHubIssuesErrors struct {
	// EnableErrors is used to enable or disable GitHub issues for errors
	Enabled bool `json:"enabled" yaml:"enabled"`
	// Labels is a list of labels to apply to issues created by driftive for errors
	Labels []string `json:"labels" yaml:"labels"`
	// MaxOpenIssues is the maximum number of open issues to have at any time for errors
	MaxOpenIssues int `json:"max_open_issues" yaml:"max_open_issues"`
	// CloseResolved is used to close resolved driftive issues for errors
	CloseResolved bool `json:"close_resolved" yaml:"close_resolved"`
}

type DriftiveRepoConfigGitHubIssues struct {
	// Enabled is used to enable or disable GitHub issues integration
	Enabled bool `json:"enabled" yaml:"enabled"`
	// CloseResolved is used to close resolved driftive issues
	CloseResolved bool `json:"close_resolved" yaml:"close_resolved"`
	// Labels is a list of labels to apply to issues created by driftive
	Labels []string `json:"labels" yaml:"labels"`
	// MaxOpenIssues is the maximum number of open issues to have at any time
	MaxOpenIssues int `json:"max_open_issues" yaml:"max_open_issues"`
	// Errors is used to configure error handling for GitHub issues
	Errors DriftiveRepoConfigGitHubIssuesErrors `json:"errors" yaml:"errors"`
}

type DriftiveRepoConfigGitHubSummary struct {
	// Enabled is used to enable or disable GitHub summary
	Enabled bool `json:"enabled" yaml:"enabled"`
	// IssueTitle is the title of the issue created by driftive for the summary
	IssueTitle string `json:"issue_title" yaml:"issue_title"`
}

type DriftiveRepoConfigGitHub struct {
	Issues       DriftiveRepoConfigGitHubIssues       `json:"issues" yaml:"issues"`
	Summary      DriftiveRepoConfigGitHubSummary      `json:"summary" yaml:"summary"`
	PullRequests DriftiveRepoConfigGithubPullRequests `json:"pull_requests" yaml:"pull_requests"`
}

type DriftiveRepoConfigGithubPullRequestsErrors struct {
	// EnableErrors is used to enable or disable GitHub pull requests for errors
	Enabled bool `json:"enabled" yaml:"enabled"`
	// Labels is a list of labels to apply to pull requests created by driftive for errors
	Labels []string `json:"labels" yaml:"labels"`
	// MaxOpenPullRequests is the maximum number of open pull requests to have at any time
	MaxOpenPullRequests int `json:"max_open_pull_requests" yaml:"max_open_pull_requests"`
	// CloseResolved is used to close resolved driftive pull requests for errors
	CloseResolved bool `json:"close_resolved" yaml:"close_resolved"`
}

type DriftiveRepoConfigGithubPullRequests struct {
	// Enabled is used to enable or disable GitHub pull requests for drift remediation
	Enabled bool `json:"enabled" yaml:"enabled"`
	// Labels is a list of labels to apply to pull requests created by driftive for drift remediation
	Labels []string `json:"labels" yaml:"labels"`
	// BaseBranch is the base branch for the pull request
	BaseBranch string `json:"base_branch" yaml:"base_branch"`
	// RemediationBranchPrefix is the prefix for the branch name created by driftive for drift remediation
	RemediationBranchPrefix string `json:"branch_prefix" yaml:"branch_prefix"`
	// Title is the title of the pull request created by driftive for drift remediation
	Title string `json:"title" yaml:"title"`
	// CloseResolved is used to close resolved driftive pull requests
	CloseResolved bool `json:"close_resolved" yaml:"close_resolved"`
	// MaxOpenPullRequests is the maximum number of open pull requests to have at any time for drift remediation
	MaxOpenPullRequests int `json:"max_open_pull_requests" yaml:"max_open_pull_requests"`
	// Errors is used to configure error handling for GitHub pull requests
	Errors DriftiveRepoConfigGithubPullRequestsErrors `json:"errors" yaml:"errors"`
}

// DriftiveRepoConfig is used to configure driftive for a repository.
// It may be defined in a .driftive.yaml file in the repository or passed via environment variable.
type DriftiveRepoConfig struct {
	AutoDiscover DriftiveRepoConfigAutoDiscover `json:"auto_discover" yaml:"auto_discover"`
	GitHub       DriftiveRepoConfigGitHub       `json:"github" yaml:"github"`
	Settings     DriftiveRepoConfigSettings     `json:"settings" yaml:"settings"`
}

// DriftiveRepoConfigSettings is used to configure driftive settings for a repository
type DriftiveRepoConfigSettings struct {
	// SkipIfOpenPR is used to skip drift notifications if there are open PRs modifying the drifted files
	SkipIfOpenPR bool `json:"skip_if_open_pr,omitempty" yaml:"skip_if_open_pr,omitempty"`
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
