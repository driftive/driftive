package git

import (
	"driftive/pkg/exec"
	"driftive/pkg/utils"
	"fmt"
	"github.com/rs/zerolog/log"
)

func CloneRepo(repoURL, branch, path string) error {
	log.Info().Msg(fmt.Sprintf("Cloning %s branch %s to %s", utils.RemoveGitRepositoryURLCredentials(repoURL), branch, path))
	_, err := exec.RunCommand("git", "clone", "-b", branch, repoURL, path)
	return err
}
