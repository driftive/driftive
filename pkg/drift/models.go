package drift

import (
	"driftive/pkg/config"
	"driftive/pkg/config/repo"
	"driftive/pkg/models"
	"driftive/pkg/utils"
	"driftive/pkg/vcs/vcstypes"
	"sync"
	"time"
)

// Stash stores required state for drift detection
type Stash struct {
	// OpenPRChangedFiles contains the list of files changed in currently open PRs
	OpenPRChangedFiles []string
	OpenIssues         []*vcstypes.VCSIssue
	OpenPrs            []*vcstypes.VCSPullRequest
}

type DriftDetector struct {
	RepoDir    string
	Projects   []models.TypedProject
	Config     *config.DriftiveConfig
	RepoConfig *repo.DriftiveRepoConfig

	workerWg  sync.WaitGroup
	results   chan DriftProjectResult
	semaphore chan struct{}

	Stash Stash
}

type DriftProjectResult struct {
	Project models.TypedProject `json:"project"`
	Drifted bool                `json:"drifted"`
	// Succeeded true if the drift analysis succeeded, even if the project had drifted.
	Succeeded  bool   `json:"succeeded"`
	InitOutput string `json:"init_output"`
	PlanOutput string `json:"plan_output"`
	// SkippedDueToPR is true if the drift was skipped because there are open PRs modifying the drifted files
	SkippedDueToPR bool `json:"skipped_due_to_pr"`
}

type DriftDetectionResult struct {
	ProjectResults []DriftProjectResult `json:"project_results"`
	TotalDrifted   int                  `json:"total_drifted"`
	TotalProjects  int                  `json:"total_projects"`
	TotalChecked   int                  `json:"total_checked"`
	Duration       time.Duration        `json:"duration"`
}

func NewDriftDetector(repoDir string, projects []models.TypedProject, cfg *config.DriftiveConfig,
	repoConfig *repo.DriftiveRepoConfig, openIssues []*vcstypes.VCSIssue, openPRChangedFiles []string, openPRs []*vcstypes.VCSPullRequest) DriftDetector {
	return DriftDetector{
		RepoDir:    repoDir,
		Projects:   projects,
		Config:     cfg,
		RepoConfig: repoConfig,
		workerWg:   sync.WaitGroup{},
		results:    nil,
		semaphore:  make(chan struct{}, utils.Max(1, cfg.Concurrency)),

		Stash: Stash{
			OpenPRChangedFiles: openPRChangedFiles,
			OpenIssues:         openIssues,
			OpenPrs:            openPRs,
		},
	}
}
