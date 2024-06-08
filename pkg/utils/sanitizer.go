package utils

import "strings"

func RemoveGitRepositoryURLCredentials(url string) string {
	atIndex := strings.Index(url, "@")
	if atIndex == -1 {
		// No credentials found
		return url
	}
	return url[atIndex+1:]
}
