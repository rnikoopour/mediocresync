package sync

import (
	"path"
	"strings"
)

// applyFilters reports whether a remote file at filePath should be included.
//
// Include groups are ANDed: a non-empty includePathFilters requires the file
// to be under at least one listed subdirectory; a non-empty includeNameFilters
// requires the basename to match at least one glob pattern.
//
// Exclude groups are ORed: the file is excluded if it is under any
// excludePathFilter subdirectory OR if its basename matches any excludeNameFilter.
func applyFilters(
	filePath, jobRemotePath string,
	includePathFilters, includeNameFilters,
	excludePathFilters, excludeNameFilters []string,
) bool {
	base := strings.TrimSuffix(jobRemotePath, "/")

	if len(includePathFilters) > 0 && !matchesAnyPath(filePath, base, includePathFilters) {
		return false
	}
	if len(includeNameFilters) > 0 && !matchesAnyName(filePath, includeNameFilters) {
		return false
	}
	if matchesAnyPath(filePath, base, excludePathFilters) {
		return false
	}
	if matchesAnyName(filePath, excludeNameFilters) {
		return false
	}
	return true
}

func matchesAnyPath(filePath, base string, subdirs []string) bool {
	for _, subdir := range subdirs {
		prefix := base + "/" + strings.Trim(subdir, "/")
		if strings.HasPrefix(filePath, prefix+"/") || filePath == prefix {
			return true
		}
	}
	return false
}

func matchesAnyName(filePath string, patterns []string) bool {
	name := path.Base(filePath)
	for _, pattern := range patterns {
		if matched, _ := path.Match(pattern, name); matched {
			return true
		}
	}
	return false
}
