package utils

import "driftive/pkg/models"

func ExtractProjectDirs(projects []models.Project) []string {
	dirs := make([]string, 0, len(projects))
	for _, project := range projects {
		dirs = append(dirs, project.Dir)
	}
	return dirs
}
