package git

import (
	"context"
	"driftive/pkg/exec"
	"driftive/pkg/utils"
	"fmt"

	"github.com/rs/zerolog/log"
)

func CloneRepo(ctx context.Context, repoURL, branch, path string) error {
	log.Info().Msg(fmt.Sprintf("Cloning %s branch %s to %s", utils.RemoveGitRepositoryURLCredentials(repoURL), branch, path))
	_, err := exec.RunCommand(ctx, "git", "clone", "-b", branch, repoURL, path)
	return err
}
