package gh

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/rs/zerolog/log"
)

type GithubActionContext struct {
	Action           string      `json:"action"`
	ActionPath       string      `json:"action_path"`
	ActionRef        string      `json:"action_ref"`
	ActionRepository string      `json:"action_repository"`
	ActionStatus     string      `json:"action_status"`
	Actor            string      `json:"actor"`
	BaseRef          string      `json:"base_ref"`
	Env              string      `json:"env"`
	Event            interface{} `json:"event"`
	EventName        string      `json:"event_name"`
	EventPath        string      `json:"event_path"`
	Path             string      `json:"path"`
	RefType          string      `json:"ref_type"`
	Repository       string      `json:"repository"`
	RepositoryOwner  string      `json:"repository_owner"`
}

// IsValid returns true if the GitHub context has all required fields
func (c *GithubActionContext) IsValid() bool {
	return c != nil && c.Repository != "" && c.RepositoryOwner != "" && c.GetRepositoryName() != ""
}

func (c *GithubActionContext) GetRepositoryName() string {
	if c == nil {
		return ""
	}
	repoName := strings.Split(c.Repository, "/")
	if len(repoName) > 0 {
		return repoName[len(repoName)-1]
	}
	return ""
}

func ParseGithubActionContext(ghContext string) (*GithubActionContext, error) {
	ghActionContext := new(GithubActionContext)
	err := json.Unmarshal([]byte(ghContext), &ghActionContext)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal github action context. %w", err)
	}
	return ghActionContext, nil
}

func ParseGHActionContextEnvVar() (*GithubActionContext, error) {
	ghContext := os.Getenv("GITHUB_CONTEXT")
	if ghContext == "" {
		return nil, fmt.Errorf("GITHUB_CONTEXT is not defined")
	}
	log.Debug().Msg("GITHUB_CONTEXT is defined. Parsing...")
	return ParseGithubActionContext(ghContext)
}
