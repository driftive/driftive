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

// /<repoPath>/path/to/file.txt -> path/to/file.txt
func removeRepoDirPrefix(repoPath string, fullFilePath string) string {
	if len(repoPath) > 0 && repoPath[len(repoPath)-1] == os.PathSeparator {
		repoPath = repoPath[:len(repoPath)-1]
	}
	return fullFilePath[len(repoPath)+1:]
}

func (d *DriftDetector) handleSkipIfContainsPRChanges(analysisResult *DriftDetectionResult) {
	if analysisResult.TotalDrifted > 0 {
		for i := range analysisResult.ProjectResults {
			projectResult := &analysisResult.ProjectResults[i]
			if projectResult.Drifted {
				if len(d.Stash.OpenPRChangedFiles) <= 0 {
					return
				}
				for _, file := range d.Stash.OpenPRChangedFiles {
					fileFolder := removeTrailingSlash(getFolder(file))
					projectFolder := removeTrailingSlash(removeRepoDirPrefix(d.Config.RepositoryPath, projectResult.Project.Dir))

					if fileFolder == projectFolder {
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
