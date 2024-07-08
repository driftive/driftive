package utils

import (
	"os"
	"path/filepath"
)

func GetBasePath() string {
	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	for _, err := os.ReadFile(filepath.Join(dir, "go.mod")); err != nil && len(dir) > 1; {
		dir = filepath.Dir(dir)
		_, err = os.ReadFile(filepath.Join(dir, "go.mod"))
	}
	if len(dir) < 2 {
		panic("No go.mod found")
	}
	return dir
}

func GetTestFile(relativePath string) []byte {
	absolutePath := filepath.Join(GetBasePath(), relativePath)
	file, err := os.ReadFile(absolutePath)
	if err != nil {
		panic(err)
	}
	return file
}
