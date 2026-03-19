package api

import (
	"net/http"
	"os"
	"path/filepath"
	"sort"
)

func localBrowse(w http.ResponseWriter, r *http.Request) {
	dir := r.URL.Query().Get("path")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "/"
		}
		dir = home
	}

	// Resolve to an absolute, clean path so the client always gets canonical paths.
	abs, err := filepath.Abs(dir)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid path")
		return
	}

	entries, err := os.ReadDir(abs)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	type entry struct {
		Name  string `json:"name"`
		Path  string `json:"path"`
		IsDir bool   `json:"is_dir"`
	}

	dirs := make([]entry, 0)
	var files []entry
	for _, e := range entries {
		// Skip hidden entries.
		if len(e.Name()) > 0 && e.Name()[0] == '.' {
			continue
		}
		full := filepath.Join(abs, e.Name())
		if e.IsDir() {
			dirs = append(dirs, entry{Name: e.Name(), Path: full, IsDir: true})
		} else {
			files = append(files, entry{Name: e.Name(), Path: full, IsDir: false})
		}
	}
	sort.Slice(dirs, func(i, j int) bool { return dirs[i].Name < dirs[j].Name })
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })

	writeJSON(w, http.StatusOK, append(dirs, files...))
}
