package github

import (
	"context"
	"fmt"
	"github.com/google/go-github/v69/github"
	"github.com/rs/zerolog/log"
	"net/http"
)

// PullRequest represents a GitHub pull request
type PullRequest struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	State  string `json:"state"`
	Files  []struct {
		Filename string `json:"filename"`
	} `json:"files"`
}

func (g *GHOps) GetAllOpenPRs(ctx context.Context) ([]PullRequest, error) {
	log.Info().Msg("Fetching all open pull requests from the repository...")
	opts := &github.PullRequestListOptions{
		State: "open",
		ListOptions: github.ListOptions{
			PerPage: 100,
			Page:    1,
		},
	}

	allPRs := make([]PullRequest, 0)
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
			allPRs = append(allPRs, PullRequest{
				Number: pr.GetNumber(),
				Title:  pr.GetTitle(),
				State:  pr.GetState(),
			})
		}

		if resp.NextPage == 0 {
			break
		}

		opts.Page = resp.NextPage
	}

	log.Info().Msgf("Fetched %d open pull requests", len(allPRs))
	return allPRs, nil
}

func (g *GHOps) GetChangedFilesForAllPRs(ctx context.Context) ([]string, error) {
	log.Info().Msg("Fetching changed files for all open pull requests...")
	allPrs, err := g.GetAllOpenPRs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get all open PRs: %w", err)
	}
	changedFiles := make([]string, 0)
	for _, pr := range allPrs {
		files, err := g.GetChangedFiles(ctx, pr.Number)
		if err != nil {
			log.Error().Msgf("Failed to get changed files for PR %d: %v", pr.Number, err)
			continue
		}
		changedFiles = append(changedFiles, files...)
	}
	log.Info().Msgf("Found %d changed files", len(changedFiles))
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
