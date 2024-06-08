package pkg

import (
	"fmt"
	"os"
	"path/filepath"
)

// DetectTerragruntProjects detects all terragrunt projects recursively in a directory
func DetectTerragruntProjects(dir string) []string {
	targetFileName := "terragrunt.hcl"
	var foldersContainingFile []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// Check if the file is a terragrunt file. Ignore root terragrunt files.
		if !info.IsDir() && info.Name() == targetFileName && path != filepath.Join(dir, targetFileName) {
			folder := filepath.Dir(path)
			foldersContainingFile = append(foldersContainingFile, folder)
		}
		return nil
	})

	if err != nil {
		fmt.Printf("Error walking the path %v: %v\n", dir, err)
		return nil
	}

	return foldersContainingFile
}
