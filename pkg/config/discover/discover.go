package discover

import (
	"driftive/pkg/config/repo"
	"driftive/pkg/models"
	"driftive/pkg/utils"
	"github.com/moby/patternmatcher"
	"github.com/rs/zerolog/log"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func executableToProjectType(executable string) models.ProjectType {
	switch executable {
	case "terraform":
		return models.Terraform
	case "tofu":
		return models.Tofu
	case "terragrunt":
		return models.Terragrunt
	default:
		log.Warn().Msgf("Unknown executable type %v", executable)
		return models.Terraform
	}
}

// DetectTerragruntProjects detects all terragrunt projects recursively in a directory
func DetectTerragruntProjects(dir string) []string {
	targetFileName := "terragrunt.hcl"
	var foldersContainingFile []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// Check if the file is a terragrunt file. Ignore root terragrunt files.
		if !info.IsDir() && info.Name() == targetFileName && path != filepath.Join(dir, targetFileName) && !isPartOfCacheFolder(path) {
			folder := filepath.Dir(path)
			foldersContainingFile = append(foldersContainingFile, folder)
		}
		return nil
	})

	if err != nil {
		log.Info().Msgf("Error walking the path %v: %v\n", dir, err)
		return nil
	}

	return foldersContainingFile
}

func isPartOfCacheFolder(dir string) bool {
	return strings.Contains(dir, ".terragrunt-cache") || strings.Contains(dir, ".terraform")
}

func AutoDiscoverProjects(rootDir string, config *repo.DriftiveRepoConfig) []models.Project {
	projs := getAllPossibleProjectPaths(rootDir, config)
	mapProjects := make(map[string]*models.Project)
	rules := config.AutoDiscover.ProjectRules

	for _, proj := range projs {
		for _, rule := range rules {
			err := filepath.Walk(proj, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}

				// Skip root dir and subdirectories
				if path == proj {
					return nil
				}
				if info.IsDir() {
					return filepath.SkipDir
				}

				match, err := filepath.Match(rule.Pattern, filepath.Base(path))
				if err != nil {
					return err
				}
				if match {
					projectType := executableToProjectType(rule.Executable)
					project := &models.Project{
						Dir:  proj,
						Type: projectType,
					}
					mapProjects[proj] = project
					return filepath.SkipAll
				}
				return nil
			})
			if err != nil {
				log.Error().Msgf("Error walking the path %v: %v\n", proj, err)
				continue
			}

			if _, ok := mapProjects[proj]; ok {
				break
			}
		}
	}

	driftiveProjects := make([]models.Project, 0, len(mapProjects))
	for _, project := range mapProjects {
		driftiveProjects = append(driftiveProjects, *project)
	}

	return driftiveProjects
}

// FilterPaths filters the paths based on inclusion and exclusion patterns
func filterPaths(paths []string, inclusions, exclusions []string) ([]string, error) {
	var filteredPaths []string
	inclPM, err := patternmatcher.New(inclusions)
	if err != nil {
		return nil, err
	}
	exclPM, err := patternmatcher.New(exclusions)
	if err != nil {
		return nil, err
	}

	for _, path := range paths {
		inclMatches, err := inclPM.MatchesOrParentMatches(path)
		if err != nil {
			return nil, err
		}

		exclMatches, err := exclPM.MatchesOrParentMatches(path)
		if err != nil {
			return nil, err
		}

		if inclMatches && !exclMatches {
			filteredPaths = append(filteredPaths, path)
		}
	}

	return filteredPaths, nil
}

// GetAllFiles returns a list of all files in the directory
func getAllFiles(root string) ([]string, error) {
	var files []string

	err := filepath.Walk(root, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return files, nil
}

func getAllPossibleProjectPaths(root string, config *repo.DriftiveRepoConfig) []string {
	allFiles, err := getAllFiles(root)
	if err != nil {
		log.Error().Msgf("Error getting all files in %v: %v", root, err)
		return nil
	}

	filteredFiles, err := filterPaths(allFiles, config.AutoDiscover.Inclusions, config.AutoDiscover.Exclusions)
	if err != nil {
		log.Error().Msgf("Error filtering files: %v\n", err)
		return nil
	}

	var projectDirs []string

	for _, file := range filteredFiles {
		stat, err := os.Stat(file)
		if err != nil {
			log.Error().Msgf("Error getting stat for file %v: %v", file, err)
			continue
		}

		if root != file && !stat.IsDir() && !utils.Contains(projectDirs, filepath.Dir(file)) {
			projectDirs = append(projectDirs, filepath.Dir(file))
		}
	}

	return projectDirs
}
