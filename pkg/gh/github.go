package gh

import (
	"encoding/json"
	"fmt"
	"os"
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
	return ParseGithubActionContext(ghContext)
}
