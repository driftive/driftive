package drift

import (
	"driftive/pkg/config"
	"driftive/pkg/config/repo"
	"driftive/pkg/exec"
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
}

type DriftDetector struct {
	RepoDir    string
	Projects   []models.TypedProject
	Config     *config.DriftiveConfig
	RepoConfig *repo.DriftiveRepoConfig

	workerWg  sync.WaitGroup
	results   chan DriftProjectResult
	semaphore chan struct{}

	// newExecutor builds the executor for a project. Defaults to exec.NewExecutor; tests
	// substitute a fake so DetectDrift can run without terraform/tofu installed.
	newExecutor func(dir string, t models.ProjectType) exec.Executor

	// OnProjectStart receives the repo-relative dir when a project's analysis begins.
	// OnProjectDone receives the finished result, whose Project.Dir is the same repo-relative
	// string. Both are optional; when nil the scan behaves exactly as if they did not exist.
	// Called from worker goroutines, so an implementation must be safe for concurrent use and
	// must not block — anything slow here serializes the scan.
	OnProjectStart func(dir string)
	OnProjectDone  func(result DriftProjectResult)

	Stash Stash
}

// Phases reported by DriftProjectResult.FailedPhase.
const (
	PhaseInit = "init"
	PhasePlan = "plan"
)

type DriftProjectResult struct {
	Project models.TypedProject `json:"project"`
	Drifted bool                `json:"drifted"`
	// Succeeded true if the drift analysis succeeded, even if the project had drifted.
	Succeeded  bool   `json:"succeeded"`
	InitOutput string `json:"init_output"`
	PlanOutput string `json:"plan_output"`
	// SkippedDueToPR is true if the drift was skipped because there are open PRs modifying the drifted files
	SkippedDueToPR bool `json:"skipped_due_to_pr"`
	// FailedPhase is PhaseInit or PhasePlan when Succeeded is false, empty otherwise.
	FailedPhase string `json:"failed_phase,omitempty"`
}

// ErrorOutput returns the output explaining why a failed project failed. Only meaningful when
// Succeeded is false — on a successful drifted project PlanOutput holds the plan, not an error.
func (r DriftProjectResult) ErrorOutput() string {
	if r.FailedPhase == PhaseInit && r.InitOutput != "" {
		return r.InitOutput
	}
	if r.PlanOutput != "" {
		return r.PlanOutput
	}
	return r.InitOutput
}

type DriftDetectionResult struct {
	ProjectResults []DriftProjectResult `json:"project_results"`
	TotalDrifted   int                  `json:"total_drifted"`
	TotalErrored   int                  `json:"total_errored"`
	TotalSkipped   int                  `json:"total_skipped"`
	TotalProjects  int                  `json:"total_projects"`
	TotalChecked   int                  `json:"total_checked"`
	Duration       time.Duration        `json:"duration"`
}

func NewDriftDetector(repoDir string, projects []models.TypedProject, cfg *config.DriftiveConfig,
	repoConfig *repo.DriftiveRepoConfig, openIssues []*vcstypes.VCSIssue, openPRChangedFiles []string) DriftDetector {
	return DriftDetector{
		RepoDir:     repoDir,
		Projects:    projects,
		Config:      cfg,
		RepoConfig:  repoConfig,
		workerWg:    sync.WaitGroup{},
		results:     nil,
		semaphore:   make(chan struct{}, utils.Max(1, cfg.Concurrency)),
		newExecutor: exec.NewExecutor,

		Stash: Stash{
			OpenPRChangedFiles: openPRChangedFiles,
			OpenIssues:         openIssues,
		},
	}
}
