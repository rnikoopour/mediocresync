// Package gitsource implements Git repository syncing.
// Each job holds a list of git_repos; this package clones or pulls each one
// into <local_dest>/<host>/<org>/<repo> and reports the current commit hash
// as the sync fingerprint.
package gitsource

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/rnikoopour/mediocresync/internal/db"
)

// RepoResult is returned by Sync and Enumerate for a single repository.
type RepoResult struct {
	Repo       *db.GitRepo
	LocalPath  string
	CommitHash string
	Err        error // non-nil if this repo could not be enumerated
}

// LocalPath returns the filesystem path where repo should be cloned:
// <localDest>/<host>/<org>/<repoName>
func LocalPath(localDest, repoURL string) (string, error) {
	u, err := url.Parse(repoURL)
	if err != nil {
		return "", fmt.Errorf("parse repo url %q: %w", repoURL, err)
	}
	// path is typically /org/repo or /org/repo.git
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 {
		return "", fmt.Errorf("repo url %q: expected at least org/repo in path", repoURL)
	}
	org := parts[len(parts)-2]
	repoName := strings.TrimSuffix(parts[len(parts)-1], ".git")
	return filepath.Join(localDest, u.Hostname(), org, repoName), nil
}

// validateAuthForURL returns a descriptive error when the auth method type is
// incompatible with the URL scheme (e.g. SSH URL with HTTP token auth).
func validateAuthForURL(repoURL string, auth transport.AuthMethod) error {
	isSSH := strings.HasPrefix(repoURL, "git@") || strings.HasPrefix(repoURL, "ssh://")
	isHTTP := strings.HasPrefix(repoURL, "https://") || strings.HasPrefix(repoURL, "http://")
	_, isHTTPAuth := auth.(*http.BasicAuth)
	_, isSSHAuth := auth.(*ssh.PublicKeys)
	if isSSH && isHTTPAuth {
		return fmt.Errorf("repo URL %q uses SSH but source is configured with token (HTTP) auth — change the URL to HTTPS or switch the source auth type to SSH key", repoURL)
	}
	if isHTTP && isSSHAuth {
		return fmt.Errorf("repo URL %q uses HTTPS but source is configured with SSH key auth — change the URL to SSH or switch the source auth type to token", repoURL)
	}
	return nil
}

// authForSource returns the go-git transport.AuthMethod for the given source.
// Returns nil (anonymous) when auth_type is empty or "none".
func authForSource(src *db.Source, plaintext string) (transport.AuthMethod, error) {
	switch src.AuthType {
	case "", db.AuthTypeNone:
		return nil, nil
	case db.AuthTypeToken:
		return &http.BasicAuth{Username: "token", Password: plaintext}, nil
	case db.AuthTypeSSHKey:
		keys, err := ssh.NewPublicKeys("git", []byte(plaintext), "")
		if err != nil {
			return nil, fmt.Errorf("parse ssh key: %w", err)
		}
		return keys, nil
	default:
		return nil, fmt.Errorf("unknown auth_type %q", src.AuthType)
	}
}

// Enumerate returns a RepoResult for each repo without modifying the filesystem.
// It clones/pulls to read the current remote HEAD commit hash.
// Per-repo errors are captured in RepoResult.Err rather than aborting the batch.
func Enumerate(ctx context.Context, src *db.Source, localDest string, repos []*db.GitRepo, credPlaintext string) ([]RepoResult, error) {
	auth, err := authForSource(src, credPlaintext)
	if err != nil {
		return nil, err
	}
	return EnumerateWithAuth(ctx, localDest, repos, auth)
}

// EnumerateWithAuth is like Enumerate but accepts a pre-built auth method,
// allowing callers that already hold a transport.AuthMethod to avoid redundant
// credential decryption.
func EnumerateWithAuth(ctx context.Context, localDest string, repos []*db.GitRepo, auth transport.AuthMethod) ([]RepoResult, error) {
	results := make([]RepoResult, 0, len(repos))
	for _, repo := range repos {
		localPath, pathErr := LocalPath(localDest, repo.URL)
		if pathErr != nil {
			results = append(results, RepoResult{Repo: repo, Err: pathErr})
			continue
		}

		hash, hashErr := currentHashWithAuth(ctx, repo, localPath, auth)
		if hashErr != nil {
			results = append(results, RepoResult{Repo: repo, LocalPath: localPath, Err: hashErr})
			continue
		}
		results = append(results, RepoResult{
			Repo:       repo,
			LocalPath:  localPath,
			CommitHash: hash,
		})
	}
	return results, nil
}

// Sync clones or pulls a single repository. Returns the resulting commit hash.
func Sync(ctx context.Context, repo *db.GitRepo, localPath string, auth transport.AuthMethod) (string, error) {
	if auth != nil {
		if err := validateAuthForURL(repo.URL, auth); err != nil {
			return "", err
		}
	}
	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", filepath.Dir(localPath), err)
	}

	r, err := git.PlainOpen(localPath)
	if errors.Is(err, git.ErrRepositoryNotExists) {
		r, err = git.PlainCloneContext(ctx, localPath, false, &git.CloneOptions{
			URL:           repo.URL,
			ReferenceName: branchRef(repo.Branch),
			SingleBranch:  true,
			Auth:          auth,
		})
		if err != nil {
			return "", fmt.Errorf("clone %s: %w", repo.URL, err)
		}
		if err := disableFileMode(r); err != nil {
			return "", fmt.Errorf("set filemode config %s: %w", repo.URL, err)
		}
	} else if err != nil {
		return "", fmt.Errorf("open %s: %w", localPath, err)
	} else {
		if err := disableFileMode(r); err != nil {
			return "", fmt.Errorf("set filemode config %s: %w", repo.URL, err)
		}
		// Fetch then hard-reset to the remote branch tip so local modifications
		// (e.g. from filesystem tools or OS metadata writes) never block the sync.
		if err := r.FetchContext(ctx, &git.FetchOptions{
			RemoteName: "origin",
			RefSpecs:   []gitconfig.RefSpec{gitconfig.RefSpec("+refs/heads/" + repo.Branch + ":refs/remotes/origin/" + repo.Branch)},
			Auth:       auth,
		}); err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
			return "", fmt.Errorf("fetch %s: %w", repo.URL, err)
		}

		ref, err := r.Reference(plumbing.NewRemoteReferenceName("origin", repo.Branch), true)
		if err != nil {
			return "", fmt.Errorf("remote ref %s: %w", repo.URL, err)
		}

		wt, err := r.Worktree()
		if err != nil {
			return "", fmt.Errorf("worktree: %w", err)
		}
		if err := wt.Reset(&git.ResetOptions{
			Commit: ref.Hash(),
			Mode:   git.HardReset,
		}); err != nil {
			return "", fmt.Errorf("reset %s: %w", repo.URL, err)
		}
	}

	head, err := r.Head()
	if err != nil {
		return "", fmt.Errorf("head: %w", err)
	}
	return head.Hash().String(), nil
}

// AuthMethod returns a pre-built transport.AuthMethod from source and plaintext credential.
// Exported so the engine can reuse it without reimporting go-git.
func AuthMethod(src *db.Source, credPlaintext string) (transport.AuthMethod, error) {
	return authForSource(src, credPlaintext)
}

// currentHashWithAuth fetches the remote and returns the tip commit hash of the
// tracked branch without modifying the working tree. Used during plan to detect
// upstream changes before deciding whether a full pull is needed.
func currentHashWithAuth(ctx context.Context, repo *db.GitRepo, localPath string, auth transport.AuthMethod) (string, error) {
	if auth != nil {
		if err := validateAuthForURL(repo.URL, auth); err != nil {
			return "", err
		}
	}
	r, err := git.PlainOpen(localPath)
	if errors.Is(err, git.ErrRepositoryNotExists) {
		// Not cloned yet — clone now so we can read HEAD.
		r, err = git.PlainCloneContext(ctx, localPath, false, &git.CloneOptions{
			URL:           repo.URL,
			ReferenceName: branchRef(repo.Branch),
			SingleBranch:  true,
			Auth:          auth,
		})
		if err != nil {
			return "", fmt.Errorf("clone: %w", err)
		}
		head, err := r.Head()
		if err != nil {
			return "", fmt.Errorf("head: %w", err)
		}
		return head.Hash().String(), nil
	} else if err != nil {
		return "", fmt.Errorf("open: %w", err)
	}

	// Repo already cloned — fetch to get the latest remote state, then read
	// the remote-tracking ref so we see upstream changes without touching the
	// working tree.
	if err := r.FetchContext(ctx, &git.FetchOptions{
		RemoteName: "origin",
		RefSpecs:   []gitconfig.RefSpec{gitconfig.RefSpec("+refs/heads/" + repo.Branch + ":refs/remotes/origin/" + repo.Branch)},
		Auth:       auth,
	}); err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return "", fmt.Errorf("fetch: %w", err)
	}

	ref, err := r.Reference(plumbing.NewRemoteReferenceName("origin", repo.Branch), true)
	if err != nil {
		return "", fmt.Errorf("remote ref: %w", err)
	}
	return ref.Hash().String(), nil
}

func branchRef(branch string) plumbing.ReferenceName {
	if branch == "" {
		branch = "main"
	}
	return plumbing.NewBranchReferenceName(branch)
}

// disableFileMode sets core.fileMode=false in the repo config so that
// filesystems without Unix permission support (e.g. exFAT) don't mark every
// file as modified due to mode differences.
func disableFileMode(r *git.Repository) error {
	cfg, err := r.Config()
	if err != nil {
		return err
	}
	cfg.Raw.Section("core").SetOption("fileMode", "false")
	return r.SetConfig(cfg)
}
