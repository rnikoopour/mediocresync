package gitsource

import (
	"path/filepath"
	"testing"
)

func TestLocalPath(t *testing.T) {
	tests := []struct {
		name      string
		localDest string
		repoURL   string
		want      string
		wantErr   bool
	}{
		{
			name:      "standard https URL",
			localDest: "/data",
			repoURL:   "https://github.com/myorg/myrepo",
			want:      filepath.Join("/data", "github.com", "myorg", "myrepo"),
		},
		{
			name:      "URL with .git suffix",
			localDest: "/data",
			repoURL:   "https://github.com/myorg/myrepo.git",
			want:      filepath.Join("/data", "github.com", "myorg", "myrepo"),
		},
		{
			name:      "deep path uses last two segments",
			localDest: "/data",
			repoURL:   "https://github.com/a/b/c/myrepo",
			want:      filepath.Join("/data", "github.com", "c", "myrepo"),
		},
		{
			name:      "ssh URL style",
			localDest: "/repos",
			repoURL:   "https://gitlab.com/team/project.git",
			want:      filepath.Join("/repos", "gitlab.com", "team", "project"),
		},
		{
			name:      "path with only one segment",
			localDest: "/data",
			repoURL:   "https://github.com/singlerepo",
			wantErr:   true,
		},
		{
			name:      "invalid URL",
			localDest: "/data",
			repoURL:   "://bad url",
			wantErr:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := LocalPath(tc.localDest, tc.repoURL)
			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error, got path %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
