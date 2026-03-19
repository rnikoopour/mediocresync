package sync

import "testing"

func TestApplyFilters(t *testing.T) {
	tests := []struct {
		name           string
		filePath       string
		jobRemotePath  string
		filters        []string
		excludeFilters []string
		want           bool
	}{
		// ── No filters ───────────────────────────────────────────────────────────
		{
			name:          "no filters includes everything",
			filePath:      "/foo/bar/file.txt",
			jobRemotePath: "/foo/bar",
			want:          true,
		},

		// ── path: filters ────────────────────────────────────────────────────────
		{
			name:          "path: matches direct child",
			filePath:      "/foo/bar/alpha/item.dat",
			jobRemotePath: "/foo/bar",
			filters:       []string{"path: alpha"},
			want:          true,
		},
		{
			name:          "path: matches deeply nested file",
			filePath:      "/foo/bar/alpha/Category/2024/item.dat",
			jobRemotePath: "/foo/bar",
			filters:       []string{"path: alpha"},
			want:          true,
		},
		{
			name:          "path: does not match sibling directory",
			filePath:      "/foo/bar/beta/record.dat",
			jobRemotePath: "/foo/bar",
			filters:       []string{"path: alpha"},
			want:          false,
		},
		{
			name:          "path: does not match file at remote root",
			filePath:      "/foo/bar/file.txt",
			jobRemotePath: "/foo/bar",
			filters:       []string{"path: alpha"},
			want:          false,
		},
		{
			name:          "path: does not match partial prefix (alpha2 ≠ alpha)",
			filePath:      "/foo/bar/alpha2/item.dat",
			jobRemotePath: "/foo/bar",
			filters:       []string{"path: alpha"},
			want:          false,
		},
		{
			name:          "path: root remote path",
			filePath:      "/alpha/item.dat",
			jobRemotePath: "/",
			filters:       []string{"path: alpha"},
			want:          true,
		},
		{
			name:          "path: trailing slash on remote path is normalised",
			filePath:      "/foo/bar/alpha/item.dat",
			jobRemotePath: "/foo/bar/",
			filters:       []string{"path: alpha"},
			want:          true,
		},
		{
			name:          "path: leading slash on subdir is normalised",
			filePath:      "/foo/bar/alpha/item.dat",
			jobRemotePath: "/foo/bar",
			filters:       []string{"path: /alpha"},
			want:          true,
		},

		// ── name: filters ────────────────────────────────────────────────────────
		{
			name:          "name: matches extension glob",
			filePath:      "/foo/bar/file.txt",
			jobRemotePath: "/foo/bar",
			filters:       []string{"name: *.txt"},
			want:          true,
		},
		{
			name:          "name: does not match different extension",
			filePath:      "/foo/bar/file.pdf",
			jobRemotePath: "/foo/bar",
			filters:       []string{"name: *.txt"},
			want:          false,
		},
		{
			name:          "name: matches file at any depth",
			filePath:      "/foo/bar/deep/nested/report.pdf",
			jobRemotePath: "/foo/bar",
			filters:       []string{"name: *.pdf"},
			want:          true,
		},
		{
			name:          "name: * does not cross / — matches basename only",
			filePath:      "/foo/bar/subdir/file.txt",
			jobRemotePath: "/foo/bar",
			filters:       []string{"name: subdir/*.txt"}, // * can't cross /
			want:          false,
		},
		{
			name:          "name: ? matches single character",
			filePath:      "/foo/bar/a.txt",
			jobRemotePath: "/foo/bar",
			filters:       []string{"name: ?.txt"},
			want:          true,
		},
		{
			name:          "name: ? does not match multiple characters",
			filePath:      "/foo/bar/ab.txt",
			jobRemotePath: "/foo/bar",
			filters:       []string{"name: ?.txt"},
			want:          false,
		},
		{
			name:          "name: dot is literal not wildcard",
			filePath:      "/foo/bar/fileXtxt",
			jobRemotePath: "/foo/bar",
			filters:       []string{"name: *.txt"},
			want:          false,
		},

		// ── Multiple filters (OR logic) ───────────────────────────────────────────
		{
			name:          "matches first of multiple path filters",
			filePath:      "/foo/bar/alpha/item.dat",
			jobRemotePath: "/foo/bar",
			filters:       []string{"path: alpha", "path: beta"},
			want:          true,
		},
		{
			name:          "matches second of multiple path filters",
			filePath:      "/foo/bar/beta/record.dat",
			jobRemotePath: "/foo/bar",
			filters:       []string{"path: alpha", "path: beta"},
			want:          true,
		},
		{
			name:          "does not match any filter",
			filePath:      "/foo/bar/gamma/album.log",
			jobRemotePath: "/foo/bar",
			filters:       []string{"path: alpha", "path: beta"},
			want:          false,
		},
		{
			name:          "path: and name: can be mixed — path matches",
			filePath:      "/foo/bar/alpha/item.dat",
			jobRemotePath: "/foo/bar",
			filters:       []string{"path: alpha", "name: *.log"},
			want:          true,
		},
		{
			name:          "path: and name: can be mixed — name matches",
			filePath:      "/foo/bar/gamma/album.log",
			jobRemotePath: "/foo/bar",
			filters:       []string{"path: alpha", "name: *.log"},
			want:          true,
		},
		{
			name:          "path: and name: can be mixed — neither matches",
			filePath:      "/foo/bar/gamma/album.csv",
			jobRemotePath: "/foo/bar",
			filters:       []string{"path: alpha", "name: *.log"},
			want:          false,
		},

		// ── Exclude filters ───────────────────────────────────────────────────────
		{
			name:           "exclude overrides include",
			filePath:       "/foo/bar/alpha/item.dat",
			jobRemotePath:  "/foo/bar",
			filters:        []string{"path: alpha"},
			excludeFilters: []string{"name: *.dat"},
			want:           false,
		},
		{
			name:           "exclude does not affect non-matching file",
			filePath:       "/foo/bar/alpha/item.bin",
			jobRemotePath:  "/foo/bar",
			filters:        []string{"path: alpha"},
			excludeFilters: []string{"name: *.dat"},
			want:           true,
		},
		{
			name:           "exclude with no include — excludes matching",
			filePath:       "/foo/bar/tmp/file.tmp",
			jobRemotePath:  "/foo/bar",
			excludeFilters: []string{"path: tmp"},
			want:           false,
		},
		{
			name:           "exclude with no include — passes non-matching",
			filePath:       "/foo/bar/docs/file.pdf",
			jobRemotePath:  "/foo/bar",
			excludeFilters: []string{"path: tmp"},
			want:           true,
		},

		// ── Whitespace and unknown keywords ───────────────────────────────────────
		{
			name:          "extra whitespace is trimmed",
			filePath:      "/foo/bar/alpha/item.dat",
			jobRemotePath: "/foo/bar",
			filters:       []string{"  path:   alpha  "},
			want:          true,
		},
		{
			name:          "unknown keyword is ignored — no match → excluded",
			filePath:      "/foo/bar/alpha/item.dat",
			jobRemotePath: "/foo/bar",
			filters:       []string{"ext: mkv"},
			want:          false,
		},
		{
			name:          "unknown keyword alongside valid filter — valid wins",
			filePath:      "/foo/bar/alpha/item.dat",
			jobRemotePath: "/foo/bar",
			filters:       []string{"ext: mkv", "path: alpha"},
			want:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := applyFilters(tt.filePath, tt.jobRemotePath, tt.filters, tt.excludeFilters)
			if got != tt.want {
				t.Errorf("applyFilters(%q, %q, %v) = %v, want %v",
					tt.filePath, tt.jobRemotePath, tt.filters, got, tt.want)
			}
		})
	}
}

func TestParsePathFilter(t *testing.T) {
	tests := []struct {
		input  string
		subdir string
		ok     bool
	}{
		{"path: alpha", "alpha", true},
		{"path:alpha", "alpha", true},
		{"path:  alpha  ", "alpha", true},
		{"path: /alpha", "/alpha", true},
		{"name: foo", "", false},
		{"alpha", "", false},
		{"", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			subdir, ok := parsePathFilter(tt.input)
			if ok != tt.ok || subdir != tt.subdir {
				t.Errorf("parsePathFilter(%q) = (%q, %v), want (%q, %v)",
					tt.input, subdir, ok, tt.subdir, tt.ok)
			}
		})
	}
}

func TestParseNameFilter(t *testing.T) {
	tests := []struct {
		input   string
		pattern string
		ok      bool
	}{
		{"name: *.txt", "*.txt", true},
		{"name:*.txt", "*.txt", true},
		{"name:  *.txt  ", "*.txt", true},
		{"path: foo", "", false},
		{"*.txt", "", false},
		{"", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			pattern, ok := parseNameFilter(tt.input)
			if ok != tt.ok || pattern != tt.pattern {
				t.Errorf("parseNameFilter(%q) = (%q, %v), want (%q, %v)",
					tt.input, pattern, ok, tt.pattern, tt.ok)
			}
		})
	}
}
