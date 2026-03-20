package sync

import "testing"

func TestFinalPath(t *testing.T) {
	cases := []struct {
		name       string
		localDest  string
		remoteRoot string
		remotePath string
		want       string
	}{
		{
			name:       "normal path",
			localDest:  "/data/downloads",
			remoteRoot: "/exports",
			remotePath: "/exports/reports/2024/jan.csv",
			want:       "/data/downloads/reports/2024/jan.csv",
		},
		{
			name:       "root remote path",
			localDest:  "/data/downloads",
			remoteRoot: "/",
			remotePath: "/reports/jan.csv",
			want:       "/data/downloads/reports/jan.csv",
		},
		{
			name:       "file at remote root",
			localDest:  "/data",
			remoteRoot: "/exports",
			remotePath: "/exports/file.txt",
			want:       "/data/file.txt",
		},
		{
			name:       "trailing slash on remote root",
			localDest:  "/data",
			remoteRoot: "/exports/",
			remotePath: "/exports/sub/file.txt",
			want:       "/data/sub/file.txt",
		},
		{
			name:       "deeply nested",
			localDest:  "/mnt/backup",
			remoteRoot: "/srv/ftp",
			remotePath: "/srv/ftp/a/b/c/d.bin",
			want:       "/mnt/backup/a/b/c/d.bin",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := finalPath(tc.localDest, tc.remoteRoot, tc.remotePath)
			if got != tc.want {
				t.Errorf("finalPath(%q, %q, %q) = %q, want %q",
					tc.localDest, tc.remoteRoot, tc.remotePath, got, tc.want)
			}
		})
	}
}

func TestStagingPath(t *testing.T) {
	got := stagingPath("/data/downloads", "/exports/reports/jan.csv")
	want := "/data/downloads/.mediocresync/jan.csv"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestStagingPathFlat(t *testing.T) {
	// Deep remote path should still produce a flat staging path (just the filename)
	got := stagingPath("/data", "/a/b/c/d/e/deep.csv")
	want := "/data/.mediocresync/deep.csv"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
