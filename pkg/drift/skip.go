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

func (d *DriftDetector) handleSkipIfContainsPRChanges(analysisResult *DriftDetectionResult) {
	log.Debug().Msgf("Handling skip if contains PR changes")
	log.Debug().Msgf("RepoPath: %s", d.Config.RepositoryPath)
	if analysisResult.TotalDrifted > 0 {
		for i := range analysisResult.ProjectResults {
			projectResult := &analysisResult.ProjectResults[i]
			if projectResult.Drifted {
				if len(d.Stash.OpenPRChangedFiles) <= 0 {
					return
				}
				for _, file := range d.Stash.OpenPRChangedFiles {
					log.Debug().Msgf("Checking project %s for file %s", projectResult.Project.Dir, file)
					fileFolder := removeTrailingSlash(getFolder(file))
					projectFolder := removeTrailingSlash(projectResult.Project.Dir)
					log.Debug().Msgf("Comparing file folder %s with project folder %s", fileFolder, projectFolder)
					if fileFolder == projectFolder {
						projectResult.SkippedDueToPR = true
						analysisResult.TotalDrifted--
						log.Info().Msgf("Marking project %s as skipped due to open PR", projectResult.Project.Dir)
						break
					}
					log.Debug().Msgf("File %s is not in project %s", file, projectResult.Project.Dir)
				}
			}
		}
	}
}
