package sync

import "testing"

func TestApplyFilters(t *testing.T) {
	tests := []struct {
		name       string
		remotePath string
		include    []string
		exclude    []string
		want       bool
	}{
		// ── No filters ──────────────────────────────────────────────────────────
		{
			name:       "no filters includes everything",
			remotePath: "/data/report.pdf",
			want:       true,
		},

		// ── Include only ─────────────────────────────────────────────────────────
		{
			name:       "include match by extension",
			remotePath: "/data/report.pdf",
			include:    []string{"*.pdf"},
			want:       true,
		},
		{
			name:       "include no match",
			remotePath: "/data/report.pdf",
			include:    []string{"*.txt"},
			want:       false,
		},
		{
			name:       "include first of multiple patterns matches",
			remotePath: "/data/report.pdf",
			include:    []string{"*.pdf", "*.txt"},
			want:       true,
		},
		{
			name:       "include second of multiple patterns matches",
			remotePath: "/data/notes.txt",
			include:    []string{"*.pdf", "*.txt"},
			want:       true,
		},
		{
			name:       "include no pattern matches",
			remotePath: "/data/image.png",
			include:    []string{"*.pdf", "*.txt"},
			want:       false,
		},

		// ── Exclude only ─────────────────────────────────────────────────────────
		{
			name:       "exclude match",
			remotePath: "/data/report.pdf",
			exclude:    []string{"*.pdf"},
			want:       false,
		},
		{
			name:       "exclude no match",
			remotePath: "/data/report.pdf",
			exclude:    []string{"*.txt"},
			want:       true,
		},
		{
			name:       "exclude first of multiple matches",
			remotePath: "/data/report.pdf",
			exclude:    []string{"*.pdf", "*.txt"},
			want:       false,
		},
		{
			name:       "exclude second of multiple matches",
			remotePath: "/data/notes.txt",
			exclude:    []string{"*.pdf", "*.txt"},
			want:       false,
		},
		{
			name:       "exclude no patterns match",
			remotePath: "/data/image.png",
			exclude:    []string{"*.pdf", "*.txt"},
			want:       true,
		},

		// ── Include + Exclude combined ───────────────────────────────────────────
		{
			name:       "include matches, exclude does not",
			remotePath: "/data/report.pdf",
			include:    []string{"*.pdf"},
			exclude:    []string{"*.txt"},
			want:       true,
		},
		{
			name:       "include matches, exclude also matches — exclude wins",
			remotePath: "/data/report.pdf",
			include:    []string{"*.pdf"},
			exclude:    []string{"*.pdf"},
			want:       false,
		},
		{
			name:       "include does not match, exclude irrelevant",
			remotePath: "/data/report.pdf",
			include:    []string{"*.txt"},
			exclude:    []string{"*.txt"},
			want:       false,
		},
		{
			name:       "include matches, excluded by prefix pattern",
			remotePath: "/data/draft_report.pdf",
			include:    []string{"*.pdf"},
			exclude:    []string{"*/draft_*"},
			want:       false,
		},

		// ── * crosses directory boundaries ───────────────────────────────────────
		{
			name:       "star matches across slashes for extension",
			remotePath: "/deep/nested/path/file.txt",
			include:    []string{"*.txt"},
			want:       true,
		},
		{
			name:       "star-only pattern matches everything",
			remotePath: "/deep/nested/anything.xyz",
			include:    []string{"*"},
			want:       true,
		},

		// ── Path-segment patterns (*/foo/* style) ────────────────────────────────
		{
			name:       "*/foo/* matches file one level inside foo",
			remotePath: "/any/foo/file.txt",
			include:    []string{"*/foo/*"},
			want:       true,
		},
		{
			name:       "*/foo/* matches file two levels inside foo",
			remotePath: "/any/foo/subdir/file.txt",
			include:    []string{"*/foo/*"},
			want:       true,
		},
		{
			name:       "*/foo/* matches foo at any depth in the path",
			remotePath: "/one/two/three/foo/file.txt",
			include:    []string{"*/foo/*"},
			want:       true,
		},
		{
			name:       "*/foo/* does not match path without foo component",
			remotePath: "/one/bar/file.txt",
			include:    []string{"*/foo/*"},
			want:       false,
		},
		{
			name:       "*/foo/* does not match foo as a filename",
			remotePath: "/one/bar/foo",
			include:    []string{"*/foo/*"},
			want:       false,
		},
		{
			name:       "exclude */tmp/* removes files inside any tmp directory",
			remotePath: "/data/tmp/scratch.txt",
			exclude:    []string{"*/tmp/*"},
			want:       false,
		},
		{
			name:       "exclude */tmp/* does not affect files outside tmp",
			remotePath: "/data/keep/file.txt",
			exclude:    []string{"*/tmp/*"},
			want:       true,
		},
		{
			name:       "*/foo/* include combined with exclude in same tree",
			remotePath: "/any/foo/draft.txt",
			include:    []string{"*/foo/*"},
			exclude:    []string{"*/draft*"},
			want:       false,
		},

		// ── ? matches single non-slash character ─────────────────────────────────
		{
			name:       "question mark matches single character",
			remotePath: "/data/a.txt",
			include:    []string{"*/?.txt"},
			want:       true,
		},
		{
			name:       "question mark matches single character deep path",
			remotePath: "/data/sub/a.txt",
			include:    []string{"*/?.txt"},
			want:       true, // * covers /data/sub, ? covers a
		},
		{
			name:       "question mark does not match multiple characters",
			remotePath: "/data/ab.txt",
			include:    []string{"*/?.txt"},
			want:       false,
		},
		{
			name:       "question mark does not match slash — no segment ends with single char before ext",
			remotePath: "/data/sub/abc.txt",
			include:    []string{"*/?.txt"},
			want:       false,
		},

		// ── Regexp metacharacter escaping ────────────────────────────────────────
		{
			name:       "dot in pattern is literal, not regexp wildcard",
			remotePath: "/data/fileXtxt",
			include:    []string{"*.txt"},
			want:       false,
		},
		{
			name:       "plus sign in pattern is literal",
			remotePath: "/data/a+b.txt",
			include:    []string{"*a+b*"},
			want:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := applyFilters(tt.remotePath, tt.include, tt.exclude)
			if got != tt.want {
				t.Errorf("applyFilters(%q, include=%v, exclude=%v) = %v, want %v",
					tt.remotePath, tt.include, tt.exclude, got, tt.want)
			}
		})
	}
}

func TestGlobToRegexp(t *testing.T) {
	tests := []struct {
		pattern string
		input   string
		want    bool
	}{
		{"*.txt", "/path/to/file.txt", true},
		{"*.txt", "/path/to/file.pdf", false},
		{"*/foo/*", "/any/foo/file.txt", true},
		{"*/foo/*", "/one/two/foo/file.txt", true},
		{"*/foo/*", "/one/bar/file.txt", false},
		{"?.txt", "a.txt", true},
		{"?.txt", "ab.txt", false},
		{"?.txt", "/a.txt", false}, // ? doesn't match /
		{"*", "/anything/at/all", true},
		{"*.tar.gz", "/backups/archive.tar.gz", true},
		{"*.tar.gz", "/backups/archive.tar.bz2", false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.input, func(t *testing.T) {
			got := globMatch(tt.pattern, tt.input)
			if got != tt.want {
				t.Errorf("globMatch(%q, %q) = %v, want %v", tt.pattern, tt.input, got, tt.want)
			}
		})
	}
}
