package sync

import (
	"path"
	"strings"
)

// applyFilters reports whether a remote file should be included in a sync run.
//
// Supported keywords:
//
//	path: <subdir>   Include files under <jobRemotePath>/<subdir>/ at any depth.
//	name: <pattern>  Include files whose base name matches the glob pattern.
//	                 Uses standard single-segment globbing: * matches any
//	                 sequence of non-/ characters, ? matches one non-/ character.
//
// Rules:
//   - If filters is empty, all files are included.
//   - If filters is non-empty, a file is included when it matches at least one
//     recognised filter (OR logic). Unrecognised keywords are silently ignored.
func applyFilters(filePath, jobRemotePath string, filters []string) bool {
	if len(filters) == 0 {
		return true
	}

	base := strings.TrimSuffix(jobRemotePath, "/")

	for _, f := range filters {
		f = strings.TrimSpace(f)

		if subdir, ok := parsePathFilter(f); ok {
			prefix := base + "/" + strings.Trim(subdir, "/")
			if strings.HasPrefix(filePath, prefix+"/") || filePath == prefix {
				return true
			}
			continue
		}

		if pattern, ok := parseNameFilter(f); ok {
			if matched, _ := path.Match(pattern, path.Base(filePath)); matched {
				return true
			}
			continue
		}
	}

	return false
}

// parsePathFilter extracts the subdir from a "path: <subdir>" filter.
func parsePathFilter(filter string) (subdir string, ok bool) {
	rest, found := strings.CutPrefix(filter, "path:")
	if !found {
		return "", false
	}
	return strings.TrimSpace(rest), true
}

// parseNameFilter extracts the glob pattern from a "name: <pattern>" filter.
func parseNameFilter(filter string) (pattern string, ok bool) {
	rest, found := strings.CutPrefix(filter, "name:")
	if !found {
		return "", false
	}
	return strings.TrimSpace(rest), true
}
