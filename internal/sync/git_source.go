package sync

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/rnikoopour/mediocresync/internal/crypto"
	"github.com/rnikoopour/mediocresync/internal/db"
	"github.com/rnikoopour/mediocresync/internal/gitsource"
)

// GitSource implements Source for Git remotes.
// No db fields — all persistence is handled by the engine via OnEvent.
// Credentials are held as a lazy-decryption closure; no plaintext is stored
// in struct fields.
type GitSource struct {
	repos       []*db.GitRepo
	buildAuth   func() (transport.AuthMethod, error) // closure over encKey + encrypted bytes
	localDest   string
	concurrency int
	appCtx      context.Context
}

// newGitSource constructs a GitSource from the job/source config.
// Credentials are held as a lazy-decryption closure over encKey + encrypted bytes;
// no plaintext is stored in struct fields.
func newGitSource(job *db.SyncJob, src *db.Source, repos []*db.GitRepo, encKey []byte, appCtx context.Context) *GitSource {
	encCred := src.AuthCredential
	authType := src.AuthType
	return &GitSource{
		repos: repos,
		buildAuth: func() (transport.AuthMethod, error) {
			var plaintext string
			if len(encCred) > 0 {
				var err error
				plaintext, err = crypto.Decrypt(encKey, encCred)
				if err != nil {
					return nil, fmt.Errorf("decrypt credential: %w", err)
				}
			}
			return gitsource.AuthMethod(&db.Source{AuthType: authType}, plaintext)
		},
		localDest:   job.LocalDest,
		concurrency: job.Concurrency,
		appCtx:      appCtx,
	}
}

// Plan fetches the current remote HEAD for each configured Git repo and returns
// which repos would be cloned/pulled or skipped — without modifying the
// working tree.
func (s *GitSource) Plan(ctx context.Context, in PlanInput) (*PlanOutput, error) {
	auth, err := s.buildAuth()
	if err != nil {
		return nil, fmt.Errorf("build auth: %w", err)
	}

	results, err := gitsource.EnumerateWithAuth(ctx, s.localDest, s.repos, auth)
	if err != nil {
		return nil, err
	}

	plan := &PlanOutput{Files: make([]PlanFile, 0, len(results))}
	for _, res := range results {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if res.Err != nil {
			plan.Files = append(plan.Files, PlanFile{
				RemotePath: res.Repo.URL,
				LocalPath:  res.LocalPath,
				Action:     "error",
				Error:      res.Err.Error(),
			})
			continue
		}
		state, _ := in.LookupState(res.Repo.URL)
		action := "copy"
		var prevHash string
		if state != nil && state.ContentHash != nil {
			prevHash = *state.ContentHash
			if prevHash == res.CommitHash {
				action = "skip"
				plan.ToSkip++
			} else {
				plan.ToCopy++
			}
		} else {
			plan.ToCopy++
		}
		plan.Files = append(plan.Files, PlanFile{
			RemotePath:         res.Repo.URL,
			LocalPath:          res.LocalPath,
			Action:             action,
			CommitHash:         res.CommitHash,
			PreviousCommitHash: prevHash,
		})
	}
	return plan, nil
}

// Sync clones or pulls all "copy" plan entries concurrently, emitting
// TransferEvents for the engine to handle. Skip and error entries are
// pre-processed by the engine before Sync is called.
//
// Sync always returns nil; per-repo errors are communicated via
// OnEvent(TransferEventFailed). A non-nil return indicates a hard setup failure
// that prevented any syncs from starting.
func (s *GitSource) Sync(ctx context.Context, in SyncInput) error {
	auth, err := s.buildAuth()
	if err != nil {
		return fmt.Errorf("build auth: %w", err)
	}

	repoByURL := make(map[string]*db.GitRepo, len(s.repos))
	for _, r := range s.repos {
		repoByURL[r.URL] = r
	}

	concurrency := max(s.concurrency, 1)
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for _, pf := range in.Plan.Files {
		if pf.Action != "copy" {
			continue
		}
		if ctx.Err() != nil {
			break
		}

		planFile := pf
		repo := repoByURL[planFile.RemotePath]
		if repo == nil {
			// Repo disappeared between plan and run; treat as a missing-branch repo.
			repo = &db.GitRepo{URL: planFile.RemotePath}
		}

		sem <- struct{}{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem }()

			in.OnEvent(TransferEvent{
				Kind:       TransferEventStarted,
				RemotePath: planFile.RemotePath,
			})

			slog.Info("git sync started", "repo", planFile.RemotePath, "local", planFile.LocalPath)
			start := time.Now()
			commitHash, syncErr := gitsource.Sync(ctx, repo, planFile.LocalPath, auth)
			if syncErr != nil {
				if errors.Is(syncErr, context.Canceled) || ctx.Err() != nil {
					in.OnEvent(TransferEvent{
						Kind:       TransferEventCanceled,
						RemotePath: planFile.RemotePath,
					})
				} else {
					slog.Error("git sync failed", "repo", planFile.RemotePath, "err", syncErr)
					in.OnEvent(TransferEvent{
						Kind:       TransferEventFailed,
						RemotePath: planFile.RemotePath,
						Error:      syncErr.Error(),
					})
				}
				return
			}

			slog.Info("git sync complete", "repo", planFile.RemotePath, "commit", commitHash)
			durationMs := time.Since(start).Milliseconds()
			in.OnEvent(TransferEvent{
				Kind:       TransferEventDone,
				RemotePath: planFile.RemotePath,
				DurationMs: &durationMs,
				CommitHash: &commitHash,
				SyncState: &db.SyncState{
					RemotePath:  planFile.RemotePath,
					ContentHash: &commitHash,
					CopiedAt:    time.Now().UTC(),
				},
			})
		}()
	}

	wg.Wait()
	return nil
}
