package main

import (
	"drifter/pkg"
	"drifter/pkg/git"
	"drifter/pkg/notification"
	"flag"
	"fmt"
	"os"
)

func validateArgs(repositoryUrl, repositoryPath, slackWebhookUrl, branch string) {
	if repositoryUrl == "" && repositoryPath == "" {
		panic("Repository URL or path is required")
	}
	if slackWebhookUrl == "" {
		panic("Slack webhook URL is required")
	}
	if branch == "" {
		panic("Branch is required")
	}
}

// determineRepositoryDir returns the repository path to use. If repositoryPath is provided, it is returned. Otherwise, the repositoryUrl is returned.
// The second return value is true if the repositoryPath should be deleted after the program finishes.
func determineRepositoryDir(repositoryUrl, repositoryPath, branch string) (string, bool) {
	if repositoryPath != "" {
		return repositoryPath, false
	}

	createdDir, err := os.MkdirTemp("", "drifter")
	if err != nil {
		panic(err)
	}

	println("Created temp dir: ", createdDir)
	err = git.CloneRepo(repositoryUrl, branch, createdDir)
	if err != nil {
		panic(err)
	}
	println("Cloned repo: ", repositoryUrl, " to ", createdDir)

	return createdDir, true
}

func main() {

	var repositoryUrl string
	var slackWebhookUrl string
	var branch string
	var repositoryPath string

	flag.StringVar(&repositoryPath, "repo-path", "", "Path to the repository. If provided, the repository will not be cloned.")
	flag.StringVar(&repositoryUrl, "repo-url", "", "e.g. https://<token>@github.com/<org>/<repo>. If repo-path is provided, this is ignored.")
	flag.StringVar(&branch, "branch", "", "Repository branch")
	flag.StringVar(&slackWebhookUrl, "slack-url", "", "Slack webhook URL")
	flag.Parse()

	validateArgs(repositoryUrl, repositoryPath, slackWebhookUrl, branch)

	repoDir, shouldDelete := determineRepositoryDir(repositoryUrl, repositoryPath, branch)
	if shouldDelete {
		println("Temp dir will be deleted after the program finishes")
		defer os.RemoveAll(repoDir)
	}

	projects := pkg.DetectTerragruntProjects(repoDir)
	println(fmt.Sprintf("Detected %d projects", len(projects)))
	driftResult := pkg.DetectDrift(repoDir, projects)

	if driftResult.TotalDrifted > 0 {
		fmt.Println("Drifted projects: ", driftResult.TotalDrifted)
		println("Sending notification to slack...")
		slack := notification.Slack{Url: slackWebhookUrl}
		slack.Send(driftResult)
	} else {
		fmt.Println("No drifts detected")
	}

	if driftResult.TotalDrifted > 0 {
		os.Exit(1)
	}

}
