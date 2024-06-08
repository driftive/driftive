package git

import (
	"drifter/pkg/exec"
	"drifter/pkg/utils"
	"fmt"
)

func CloneRepo(repoURL, branch, path string) error {
	println(fmt.Sprintf("Cloning %s branch %s to %s", utils.RemoveGitRepositoryURLCredentials(repoURL), branch, path))
	_, err := exec.RunCommand("git", "clone", "-b", branch, repoURL, path)
	return err
}
