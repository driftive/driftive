package drift

import (
	"github.com/rs/zerolog/log"
	"os"
	"path/filepath"
)

// getFolder returns the folder path from the file path (e.g. /path/to/file.txt -> /path/to)
func getFolder(file string) string {
	dir, _ := filepath.Split(file)
	return dir
}

func removeTrailingSlash(path string) string {
	if len(path) > 0 && path[len(path)-1] == os.PathSeparator {
		return path[:len(path)-1]
	}
	return path
}

func removeRepoDirPrefix(repoPath string, fullFilePath string) string {
	// Remove the repoPath prefix from the fullFilePath
	if len(repoPath) > 0 {
		return fullFilePath[len(repoPath)+1:]
	}
	return fullFilePath
}

func (d *DriftDetector) handleSkipIfContainsPRChanges(analysisResult *DriftDetectionResult) {
	if analysisResult.TotalDrifted > 0 && d.Config.GithubContext.IsValid() && d.Config.GithubToken != "" {
		for _, projectResult := range analysisResult.ProjectResults {
			if projectResult.Drifted {
				if len(d.Stash.OpenPRChangedFiles) <= 0 {
					return
				}
				for _, file := range d.Stash.OpenPRChangedFiles {
					if removeTrailingSlash(getFolder(file)) == removeTrailingSlash(removeRepoDirPrefix(d.Config.RepositoryPath, projectResult.Project.Dir)) {
						projectResult.SkippedDueToPR = true
						analysisResult.TotalDrifted--
						log.Warn().Msgf("Marking project %s as skipped due to open PR", projectResult.Project.Dir)
						break
					}
				}
			}
		}
	}
}
