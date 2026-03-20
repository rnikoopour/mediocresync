package sync

import "testing"

func TestApplyFilters(t *testing.T) {
	tests := []struct {
		name               string
		filePath           string
		jobRemotePath      string
		includePathFilters []string
		includeNameFilters []string
		excludePathFilters []string
		excludeNameFilters []string
		want               bool
	}{
		// ── No filters ───────────────────────────────────────────────────────────
		{
			name:          "no filters includes everything",
			filePath:      "/foo/bar/file.txt",
			jobRemotePath: "/foo/bar",
			want:          true,
		},

		// ── includePathFilters ───────────────────────────────────────────────────
		{
			name:               "includePathFilters: matches direct child",
			filePath:           "/foo/bar/alpha/item.dat",
			jobRemotePath:      "/foo/bar",
			includePathFilters: []string{"alpha"},
			want:               true,
		},
		{
			name:               "includePathFilters: matches deeply nested file",
			filePath:           "/foo/bar/alpha/Category/2024/item.dat",
			jobRemotePath:      "/foo/bar",
			includePathFilters: []string{"alpha"},
			want:               true,
		},
		{
			name:               "includePathFilters: does not match sibling directory",
			filePath:           "/foo/bar/beta/record.dat",
			jobRemotePath:      "/foo/bar",
			includePathFilters: []string{"alpha"},
			want:               false,
		},
		{
			name:               "includePathFilters: does not match file at remote root",
			filePath:           "/foo/bar/file.txt",
			jobRemotePath:      "/foo/bar",
			includePathFilters: []string{"alpha"},
			want:               false,
		},
		{
			name:               "includePathFilters: does not match partial prefix (alpha2 != alpha)",
			filePath:           "/foo/bar/alpha2/item.dat",
			jobRemotePath:      "/foo/bar",
			includePathFilters: []string{"alpha"},
			want:               false,
		},
		{
			name:               "includePathFilters: root remote path",
			filePath:           "/alpha/item.dat",
			jobRemotePath:      "/",
			includePathFilters: []string{"alpha"},
			want:               true,
		},
		{
			name:               "includePathFilters: trailing slash on remote path is normalised",
			filePath:           "/foo/bar/alpha/item.dat",
			jobRemotePath:      "/foo/bar/",
			includePathFilters: []string{"alpha"},
			want:               true,
		},
		{
			name:               "includePathFilters: leading slash on subdir is normalised",
			filePath:           "/foo/bar/alpha/item.dat",
			jobRemotePath:      "/foo/bar",
			includePathFilters: []string{"/alpha"},
			want:               true,
		},

		// ── includeNameFilters ───────────────────────────────────────────────────
		{
			name:               "includeNameFilters: matches extension glob",
			filePath:           "/foo/bar/file.txt",
			jobRemotePath:      "/foo/bar",
			includeNameFilters: []string{"*.txt"},
			want:               true,
		},
		{
			name:               "includeNameFilters: does not match different extension",
			filePath:           "/foo/bar/file.pdf",
			jobRemotePath:      "/foo/bar",
			includeNameFilters: []string{"*.txt"},
			want:               false,
		},
		{
			name:               "includeNameFilters: matches file at any depth",
			filePath:           "/foo/bar/deep/nested/report.pdf",
			jobRemotePath:      "/foo/bar",
			includeNameFilters: []string{"*.pdf"},
			want:               true,
		},
		{
			name:               "includeNameFilters: * does not cross / - matches basename only",
			filePath:           "/foo/bar/subdir/file.txt",
			jobRemotePath:      "/foo/bar",
			includeNameFilters: []string{"subdir/*.txt"},
			want:               false,
		},
		{
			name:               "includeNameFilters: ? matches single character",
			filePath:           "/foo/bar/a.txt",
			jobRemotePath:      "/foo/bar",
			includeNameFilters: []string{"?.txt"},
			want:               true,
		},
		{
			name:               "includeNameFilters: ? does not match multiple characters",
			filePath:           "/foo/bar/ab.txt",
			jobRemotePath:      "/foo/bar",
			includeNameFilters: []string{"?.txt"},
			want:               false,
		},
		{
			name:               "includeNameFilters: dot is literal not wildcard",
			filePath:           "/foo/bar/fileXtxt",
			jobRemotePath:      "/foo/bar",
			includeNameFilters: []string{"*.txt"},
			want:               false,
		},

		// ── Multiple includePathFilters (OR within group) ─────────────────────────
		{
			name:               "matches first of multiple includePathFilters",
			filePath:           "/foo/bar/alpha/item.dat",
			jobRemotePath:      "/foo/bar",
			includePathFilters: []string{"alpha", "beta"},
			want:               true,
		},
		{
			name:               "matches second of multiple includePathFilters",
			filePath:           "/foo/bar/beta/record.dat",
			jobRemotePath:      "/foo/bar",
			includePathFilters: []string{"alpha", "beta"},
			want:               true,
		},
		{
			name:               "does not match any includePathFilters",
			filePath:           "/foo/bar/gamma/album.log",
			jobRemotePath:      "/foo/bar",
			includePathFilters: []string{"alpha", "beta"},
			want:               false,
		},

		// ── AND semantics between include groups ──────────────────────────────────
		{
			name:               "path+name both match - included",
			filePath:           "/foo/bar/alpha/item.dat",
			jobRemotePath:      "/foo/bar",
			includePathFilters: []string{"alpha"},
			includeNameFilters: []string{"*.dat"},
			want:               true,
		},
		{
			name:               "path matches but name does not - excluded",
			filePath:           "/foo/bar/alpha/item.bin",
			jobRemotePath:      "/foo/bar",
			includePathFilters: []string{"alpha"},
			includeNameFilters: []string{"*.dat"},
			want:               false,
		},
		{
			name:               "name matches but path does not - excluded",
			filePath:           "/foo/bar/beta/entry.dat",
			jobRemotePath:      "/foo/bar",
			includePathFilters: []string{"alpha"},
			includeNameFilters: []string{"*.dat"},
			want:               false,
		},
		{
			name:               "neither path nor name matches - excluded",
			filePath:           "/foo/bar/gamma/album.log",
			jobRemotePath:      "/foo/bar",
			includePathFilters: []string{"alpha"},
			includeNameFilters: []string{"*.dat"},
			want:               false,
		},

		// ── Exclude filters ───────────────────────────────────────────────────────
		{
			name:               "excludePathFilters overrides include",
			filePath:           "/foo/bar/alpha/item.dat",
			jobRemotePath:      "/foo/bar",
			includePathFilters: []string{"alpha"},
			excludePathFilters: []string{"alpha"},
			want:               false,
		},
		{
			name:               "excludeNameFilters overrides include",
			filePath:           "/foo/bar/alpha/item.dat",
			jobRemotePath:      "/foo/bar",
			includePathFilters: []string{"alpha"},
			excludeNameFilters: []string{"*.dat"},
			want:               false,
		},
		{
			name:               "excludeNameFilters does not affect non-matching file",
			filePath:           "/foo/bar/alpha/item.bin",
			jobRemotePath:      "/foo/bar",
			includePathFilters: []string{"alpha"},
			excludeNameFilters: []string{"*.dat"},
			want:               true,
		},
		{
			name:               "excludePathFilters with no include - excludes matching",
			filePath:           "/foo/bar/tmp/file.tmp",
			jobRemotePath:      "/foo/bar",
			excludePathFilters: []string{"tmp"},
			want:               false,
		},
		{
			name:               "excludePathFilters with no include - passes non-matching",
			filePath:           "/foo/bar/docs/file.pdf",
			jobRemotePath:      "/foo/bar",
			excludePathFilters: []string{"tmp"},
			want:               true,
		},
		{
			name:               "excludeNameFilters with no include - excludes matching",
			filePath:           "/foo/bar/item.dat",
			jobRemotePath:      "/foo/bar",
			excludeNameFilters: []string{"*.dat"},
			want:               false,
		},
		{
			name:               "excludeNameFilters with no include - passes non-matching",
			filePath:           "/foo/bar/item.bin",
			jobRemotePath:      "/foo/bar",
			excludeNameFilters: []string{"*.dat"},
			want:               true,
		},

	// ── Glob path filters (**) ───────────────────────────────────────────────
	{
		name:               "glob includePathFilter: **/*alpha* matches direct child containing alpha",
		filePath:           "/foo/bar/sub_alpha_string/item.dat",
		jobRemotePath:      "/foo/bar",
		includePathFilters: []string{"**/*alpha*"},
		want:               true,
	},
	{
		name:               "glob includePathFilter: **/*alpha* matches deeply nested file under alpha dir",
		filePath:           "/foo/bar/sub_alpha_string/deep/nested/item.dat",
		jobRemotePath:      "/foo/bar",
		includePathFilters: []string{"**/*alpha*"},
		want:               true,
	},
	{
		name:               "glob includePathFilter: **/*alpha* matches alpha dir at non-root depth",
		filePath:           "/foo/bar/category/sub_alpha_string/item.dat",
		jobRemotePath:      "/foo/bar",
		includePathFilters: []string{"**/*alpha*"},
		want:               true,
	},
	{
		name:               "glob includePathFilter: **/*alpha* does not match file with no alpha in path",
		filePath:           "/foo/bar/beta/item.dat",
		jobRemotePath:      "/foo/bar",
		includePathFilters: []string{"**/*alpha*"},
		want:               false,
	},
	{
		name:               "glob includePathFilter: **/tmp matches tmp dir at any depth",
		filePath:           "/foo/bar/a/b/tmp/item.dat",
		jobRemotePath:      "/foo/bar",
		includePathFilters: []string{"**/tmp"},
		want:               true,
	},
	{
		name:               "glob includePathFilter: **/tmp does not match tmp2",
		filePath:           "/foo/bar/tmp2/item.dat",
		jobRemotePath:      "/foo/bar",
		includePathFilters: []string{"**/tmp"},
		want:               false,
	},
	{
		name:               "glob excludePathFilter: **/*alpha* excludes file under alpha dir",
		filePath:           "/foo/bar/sub_alpha_string/item.dat",
		jobRemotePath:      "/foo/bar",
		excludePathFilters: []string{"**/*alpha*"},
		want:               false,
	},
	{
		name:               "glob excludePathFilter: **/*alpha* does not exclude unrelated file",
		filePath:           "/foo/bar/beta/item.dat",
		jobRemotePath:      "/foo/bar",
		excludePathFilters: []string{"**/*alpha*"},
		want:               true,
	},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := applyFilters(
				tt.filePath, tt.jobRemotePath,
				tt.includePathFilters, tt.includeNameFilters,
				tt.excludePathFilters, tt.excludeNameFilters,
			)
			if got != tt.want {
				t.Errorf("applyFilters(%q, %q, path=%v, name=%v, exclPath=%v, exclName=%v) = %v, want %v",
					tt.filePath, tt.jobRemotePath,
					tt.includePathFilters, tt.includeNameFilters,
					tt.excludePathFilters, tt.excludeNameFilters,
					got, tt.want)
			}
		})
	}
}
