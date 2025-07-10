package github

import (
	"context"
	"driftive/pkg/notification/github/types"
	"driftive/pkg/vcs/vcstypes"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/go-github/v73/github"
	"github.com/rs/zerolog/log"
)

// PullRequest represents a GitHub pull request
type PullRequest struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	State  string `json:"state"`
	Files  []struct {
		Filename string `json:"filename"`
	} `json:"files"`
	Url    string   `json:"url"`
	Base   string   `json:"base"`
	Labels []string `json:"labels"`
}

func (g *GHOps) GetAllOpenPRs(ctx context.Context) ([]*vcstypes.VCSPullRequest, error) {
	log.Info().Msg("Fetching all open pull requests from the repository...")
	opts := &github.PullRequestListOptions{
		State: "open",
		ListOptions: github.ListOptions{
			PerPage: 100,
			Page:    1,
		},
	}

	allOpenPrs := make([]*vcstypes.VCSPullRequest, 0)
	for {
		prs, resp, err := g.ghClient.PullRequests.List(ctx, g.config.GithubContext.RepositoryOwner, g.config.GithubContext.GetRepositoryName(), opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list PRs: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("failed to get PRs: status code %d", resp.StatusCode)
		}

		for _, pr := range prs {
			allOpenPrs = append(allOpenPrs, &vcstypes.VCSPullRequest{
				Number: pr.GetNumber(),
				Title:  pr.GetTitle(),
				State:  pr.GetState(),
				Url:    pr.GetHTMLURL(),
				Body:   pr.GetBody(),
			})
		}

		if resp.NextPage == 0 {
			break
		}

		opts.Page = resp.NextPage
	}

	log.Info().Msgf("Fetched %d open pull requests", len(allOpenPrs))
	return allOpenPrs, nil
}

func (g *GHOps) GetChangedFilesForAllOpenPrs(ctx context.Context, allOpenPrs []*vcstypes.VCSPullRequest) ([]string, error) {
	log.Info().Msg("Fetching changed files for all open pull requests...")
	changedFiles := make([]string, 0)
	for _, pr := range allOpenPrs {
		files, err := g.GetChangedFiles(ctx, pr.Number)
		if err != nil {
			log.Error().Msgf("Failed to get changed files for PR %d: %v", pr.Number, err)
			continue
		}
		changedFiles = append(changedFiles, files...)
	}
	log.Info().Msgf("Found %d changed files", len(changedFiles))

	log.Debug().Msg("Changed files:")
	for _, file := range changedFiles {
		log.Debug().Msgf("- %s", file)
	}

	return changedFiles, nil
}

func (g *GHOps) GetChangedFiles(ctx context.Context, prNumber int) ([]string, error) {
	opts := &github.ListOptions{
		Page:    1,
		PerPage: 100,
	}
	allFiles := make([]string, 0)
	for {
		commitFiles, filesResp, err := g.ghClient.PullRequests.ListFiles(ctx, g.config.GithubContext.RepositoryOwner, g.config.GithubContext.GetRepositoryName(), prNumber, opts)
		if err != nil {
			log.Error().Msgf("Failed to list files for PR %d: %v", prNumber, err)
			return []string{}, nil
		}
		defer filesResp.Body.Close()
		for _, prFile := range commitFiles {
			allFiles = append(allFiles, prFile.GetFilename())
		}
		if filesResp.NextPage == 0 {
			break
		}
	}

	return allFiles, nil
}

func (g *GHOps) BranchExists(ctx context.Context, branchName string) (bool, error) {
	ownerRepo := strings.Split(g.config.GithubContext.Repository, "/")
	if len(ownerRepo) != 2 {
		return false, fmt.Errorf("invalid repository name")
	}

	_, resp, err := g.ghClient.Git.GetRef(ctx, ownerRepo[0], ownerRepo[1], "heads/"+branchName)
	if err != nil {
		if resp.StatusCode == 404 {
			return false, nil // Branch does not exist
		}
		return false, err // Other error
	}
	return true, nil // Branch exists
}

func (g *GHOps) CreateBranch(ctx context.Context, branchName string) error {
	// Use the default branch from config or fallback to "main"
	baseBranch := g.config.Branch
	if baseBranch == "" {
		baseBranch = "main" // Default fallback
	}

	ownerRepo := strings.Split(g.config.GithubContext.Repository, "/")
	if len(ownerRepo) != 2 {
		return fmt.Errorf("invalid repository name")
	}

	ref, _, err := g.ghClient.Git.GetRef(ctx, ownerRepo[0], ownerRepo[1], "heads/"+baseBranch)
	if err != nil {
		return fmt.Errorf("error getting base branch reference: %w", err)
	}
	newRef := &github.Reference{
		Ref:    github.Ptr("refs/heads/" + branchName),
		Object: &github.GitObject{SHA: ref.Object.SHA},
	}
	_, _, err = g.ghClient.Git.CreateRef(ctx, ownerRepo[0], ownerRepo[1], newRef)
	if err != nil {
		return fmt.Errorf("error creating new branch: %w", err)
	}
	log.Debug().Msgf("Created branch %s in repository %s/%s", branchName, ownerRepo[0], ownerRepo[1])
	return nil
}

func (g *GHOps) AddFileToBranch(
	ctx context.Context,
	branchName string,
	filePath string,
	fileContent string,
	commitMessage string) error {
	ownerRepo := strings.Split(g.config.GithubContext.Repository, "/")
	if len(ownerRepo) != 2 {
		return fmt.Errorf("invalid repository name")
	}

	_, _, err := g.ghClient.Repositories.CreateFile(ctx, ownerRepo[0], ownerRepo[1], filePath, &github.RepositoryContentFileOptions{
		Message: github.Ptr(commitMessage),
		Content: []byte(fileContent),
		Branch:  github.Ptr(branchName),
	})
	if err != nil {
		return fmt.Errorf("error adding file to branch: %w", err)
	}

	log.Debug().Msgf("Added file %s to branch %s in repository %s/%s", filePath, branchName, ownerRepo[0], ownerRepo[1])
	return nil
}

func (g *GHOps) addPullRequestLabels(ctx context.Context, owner string, repo string, pullRequest *github.PullRequest, labels []string) error {

	// get the existing labels for the pull request
	existingLabels, _, err := g.ghClient.Issues.ListLabelsByIssue(ctx, owner, repo, pullRequest.GetNumber(), nil)
	if err != nil {
		return fmt.Errorf("failed to list existing labels for pull request %d: %w", pullRequest.GetNumber(), err)
	}

	if len(existingLabels) == 0 {
		_, _, err = g.ghClient.Issues.AddLabelsToIssue(ctx, owner, repo, pullRequest.GetNumber(), labels)
		if err != nil {
			return fmt.Errorf("failed to add labels '%v' to pull request %d: %w", labels, pullRequest.GetNumber(), err)
		}

	} else {
		for _, existingLabel := range existingLabels {
			for _, label := range labels {
				if existingLabel.GetName() == label {
					continue // Skip if label already exists
				} else {
					_, _, err = g.ghClient.Issues.AddLabelsToIssue(ctx, owner, repo, pullRequest.GetNumber(), []string{label})
					if err != nil {
						return fmt.Errorf("failed to add label '%s' to pull request %d: %w", label, pullRequest.GetNumber(), err)
					}
				}
			}
		}
	}
	return nil
}

func (g *GHOps) CreateOrUpdatePullRequest(
	ctx context.Context,
	driftivePullRequest types.GithubPullRequest,
	updateOnly bool) vcstypes.CreateOrUpdatePullRequestResult {

	ownerRepo := strings.Split(g.config.GithubContext.Repository, "/")
	owner := ownerRepo[0]
	repo := ownerRepo[1]
	if len(ownerRepo) != 2 {
		log.Error().Msg("Invalid repository name")
		return vcstypes.CreateOrUpdatePullRequestResult{
			Created:     false,
			RateLimited: false,
			PullRequest: nil,
		}
	}

	// Return early if we're rate limited
	if updateOnly {
		log.Warn().Msgf("Max number of open pull requests reached. Skipping pull request creation for project %s (repo: %s/%s)",
			driftivePullRequest.Project.Dir,
			owner,
			repo)
		return vcstypes.CreateOrUpdatePullRequestResult{
			Created:     false,
			RateLimited: true,
			PullRequest: nil,
		}
	}

	// Create a new branch
	err := g.CreateBranch(ctx, driftivePullRequest.Branch)
	if err != nil {
		log.Error().Msgf("Failed to create branch %s: %v", driftivePullRequest.Branch, err)
		return vcstypes.CreateOrUpdatePullRequestResult{
			Created:     false,
			RateLimited: false,
			PullRequest: nil,
		}
	}

	// Add remediation file to the new branch
	fileContent := "driftive remediation " + driftivePullRequest.Time.UTC().Format(time.UnixDate) + "\n"
	commitContent := fmt.Sprintf("Adds driftive remediation file for project %s", driftivePullRequest.Project.Dir)
	err = g.AddFileToBranch(ctx, driftivePullRequest.Branch, "driftive-remediation.txt", fileContent, commitContent)
	if err != nil {
		log.Error().Msgf("Failed to add file to branch %s: %v", driftivePullRequest.Branch, err)
		// Clean up branch if file addition fails
		_, err := g.ghClient.Git.DeleteRef(ctx, owner, repo, "heads/"+driftivePullRequest.Branch)
		if err != nil {
			log.Error().Msgf("Failed to delete branch %s after file addition failure: %v", driftivePullRequest.Branch, err)
		}
		return vcstypes.CreateOrUpdatePullRequestResult{
			Created:     false,
			RateLimited: false,
			PullRequest: nil,
		}
	}

	// Create a new pull request
	newPR := &github.NewPullRequest{
		Title: github.Ptr(driftivePullRequest.Title),
		Head:  github.Ptr(driftivePullRequest.Branch),
		Base:  github.Ptr(driftivePullRequest.Base),
		Body:  github.Ptr(driftivePullRequest.Body),
	}

	createdPR, _, err := g.ghClient.PullRequests.Create(ctx, ownerRepo[0], ownerRepo[1], newPR)
	if err != nil {
		log.Error().Msgf("Failed to create pull request: %v", err)
		return vcstypes.CreateOrUpdatePullRequestResult{
			Created:     false,
			RateLimited: false,
			PullRequest: nil,
		}
	} else {
		g.addPullRequestLabels(ctx, owner, repo, createdPR, driftivePullRequest.Labels)
	}

	return vcstypes.CreateOrUpdatePullRequestResult{
		Created:     true,
		RateLimited: false,
		PullRequest: &vcstypes.VCSPullRequest{
			Number: createdPR.GetNumber(),
			Title:  createdPR.GetTitle(),
			Url:    createdPR.GetHTMLURL(),
		},
	}
}

func (g *GHOps) CreatePullRequestComment(
	ctx context.Context,
	pullRequestNumber int,
	comment string) error {
	owner := g.config.GithubContext.RepositoryOwner
	repo := g.config.GithubContext.GetRepositoryName()

	log.Info().Msgf("Creating comment on pull request #%d: %s", pullRequestNumber, comment)
	_, resp, err := g.ghClient.PullRequests.CreateComment(ctx, owner, repo, pullRequestNumber, &github.PullRequestComment{
		Body: github.Ptr(comment),
	})
	if err != nil {
		log.Error().Msgf("Failed to create comment on pull request #%d: %v", pullRequestNumber, err)
		return err
	}
	defer resp.Body.Close()

	return nil
}

func (g *GHOps) ClosePullRequest(ctx context.Context, pullRequestNumber int) error {
	owner := g.config.GithubContext.RepositoryOwner
	repo := g.config.GithubContext.GetRepositoryName()

	_, prUpdateResponse, err := g.ghClient.PullRequests.Edit(ctx, owner, repo, pullRequestNumber, &github.PullRequest{
		State: github.Ptr("closed"),
	})
	if err != nil {
		log.Error().Msgf("Failed to close pull request #%d: %v", pullRequestNumber, err)
		return err
	}
	defer prUpdateResponse.Body.Close()
	return nil
}
