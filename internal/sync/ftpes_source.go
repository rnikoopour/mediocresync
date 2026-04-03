package sync

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/rnikoopour/mediocresync/internal/crypto"
	"github.com/rnikoopour/mediocresync/internal/db"
	"github.com/rnikoopour/mediocresync/internal/ftpes"
)

// newFTPESSource constructs an FTPESSource from the job/source config.
// Credentials are held as a lazy-decryption closure over encKey + encrypted bytes;
// no plaintext is stored in struct fields.
func newFTPESSource(job *db.SyncJob, src *db.Source, encKey []byte, appCtx context.Context) *FTPESSource {
	encPwd := src.Password
	return &FTPESSource{
		host:               src.Host,
		port:               src.Port,
		skipTLSVerify:      src.SkipTLSVerify,
		enableEPSV:         src.EnableEPSV,
		username:           src.Username,
		getPassword: func() (string, error) {
			return crypto.Decrypt(encKey, encPwd)
		},
		remotePath:         job.RemotePath,
		localDest:          job.LocalDest,
		includePathFilters: job.IncludePathFilters,
		includeNameFilters: job.IncludeNameFilters,
		excludePathFilters: job.ExcludePathFilters,
		excludeNameFilters: job.ExcludeNameFilters,
		concurrency:        job.Concurrency,
		retryAttempts:      job.RetryAttempts,
		retryDelaySeconds:  job.RetryDelaySeconds,
		appCtx:             appCtx,
	}
}

// FTPESSource implements Source for FTPES remotes.
// No db fields — all persistence is handled by the engine via OnEvent.
// Credentials are held as a lazy-decryption closure; no plaintext is stored
// in struct fields.
type FTPESSource struct {
	host               string
	port               int
	skipTLSVerify      bool
	enableEPSV         bool
	username           string
	getPassword        func() (string, error) // closure over encKey + encrypted bytes
	remotePath         string
	localDest          string
	includePathFilters []string
	includeNameFilters []string
	excludePathFilters []string
	excludeNameFilters []string
	concurrency        int
	retryAttempts      int
	retryDelaySeconds  int
	appCtx             context.Context
}

// Plan connects to the FTPES server, walks the remote tree, and returns which
// files would be copied or skipped — without downloading anything.
func (s *FTPESSource) Plan(ctx context.Context, in PlanInput) (*PlanOutput, error) {
	password, err := s.getPassword()
	if err != nil {
		return nil, fmt.Errorf("decrypt password: %w", err)
	}

	cb := in.Progress
	if cb == nil {
		cb = func(_, _ int) {}
	}

	client, err := ftpes.Dial(s.host, s.port, s.skipTLSVerify, s.enableEPSV)
	if err != nil {
		return nil, fmt.Errorf("dial: %w", err)
	}
	defer client.Close()

	if err := client.Login(s.username, password); err != nil {
		return nil, fmt.Errorf("login: %w", err)
	}

	base := strings.TrimSuffix(s.remotePath, "/")
	pruner := makePruner(base, s.includePathFilters)
	remoteFiles, err := client.WalkWithProgress(s.remotePath, pruner, cb)
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", s.remotePath, err)
	}

	result := &PlanOutput{Files: make([]PlanFile, 0, len(remoteFiles))}
	for _, f := range remoteFiles {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if !applyFilters(f.Path, s.remotePath, s.includePathFilters, s.includeNameFilters, s.excludePathFilters, s.excludeNameFilters) {
			continue
		}
		state, _ := in.LookupState(f.Path)
		action := "copy"
		if Matches(state, f) {
			action = "skip"
			result.ToSkip++
		} else {
			result.ToCopy++
		}
		result.Files = append(result.Files, PlanFile{
			RemotePath: f.Path,
			LocalPath:  finalPath(s.localDest, s.remotePath, f.Path),
			SizeBytes:  f.Size,
			MTime:      f.MTime,
			Action:     action,
		})
	}

	// Sort to match tree view order so the engine receives files correctly ordered.
	result.Files = sortPlanFiles(result.Files, s.remotePath)
	return result, nil
}

// Sync downloads all "copy" plan entries concurrently, emitting TransferEvents
// for the engine to handle. Skip and error entries are pre-processed by the
// engine before Sync is called.
//
// Sync always returns nil; per-file errors are communicated via
// OnEvent(TransferEventFailed). A non-nil return indicates a hard setup failure
// that prevented any transfers from starting.
func (s *FTPESSource) Sync(ctx context.Context, in SyncInput) error {
	password, err := s.getPassword()
	if err != nil {
		return fmt.Errorf("decrypt password: %w", err)
	}

	if err := ensureStagingDir(s.localDest); err != nil {
		return fmt.Errorf("staging dir: %w", err)
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
		sem <- struct{}{}

		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem }()

			remote := ftpes.RemoteFile{Path: planFile.RemotePath, Size: planFile.SizeBytes, MTime: planFile.MTime}

			tryOnce := func() error {
				c, err := ftpes.Dial(s.host, s.port, s.skipTLSVerify, s.enableEPSV)
				if err != nil {
					return err
				}
				defer c.Close()
				if err := c.Login(s.username, password); err != nil {
					return err
				}
				return s.downloadFile(ctx, c, remote, in.OnEvent)
			}

			slog.Info("transfer started", "src", remote.Path, "dst", finalPath(s.localDest, s.remotePath, remote.Path), "size", remote.Size)
			maxAttempts := max(s.retryAttempts, 1)
			var lastErr error
			for attempt := 1; attempt <= maxAttempts; attempt++ {
				if ctx.Err() != nil {
					lastErr = ctx.Err()
					break
				}
				if attempt > 1 {
					slog.Warn("retrying transfer", "src", remote.Path, "dst", finalPath(s.localDest, s.remotePath, remote.Path), "attempt", attempt, "err", lastErr)
					select {
					case <-time.After(time.Duration(s.retryDelaySeconds) * time.Second):
					case <-ctx.Done():
						lastErr = ctx.Err()
					}
					if ctx.Err() != nil {
						break
					}
				}
				if lastErr = tryOnce(); lastErr == nil {
					break
				}
				if ctx.Err() != nil {
					break
				}
			}

			if lastErr != nil {
				os.Remove(stagingPath(s.localDest, remote.Path))
				slog.Error("transfer failed", "src", remote.Path, "dst", finalPath(s.localDest, s.remotePath, remote.Path), "err", lastErr)
				errMsg := lastErr.Error()
				if errors.Is(lastErr, errTransferStalled) {
					errMsg = "transfer stalled: no data received"
				} else if errors.Is(lastErr, context.Canceled) || ctx.Err() != nil {
					if s.appCtx.Err() != nil {
						errMsg = "canceled by server"
					} else {
						errMsg = "canceled by client"
					}
				}
				in.OnEvent(TransferEvent{
					Kind:       TransferEventFailed,
					RemotePath: remote.Path,
					SizeBytes:  remote.Size,
					Error:      errMsg,
				})
				return
			}

			slog.Info("transfer complete", "src", remote.Path, "dst", finalPath(s.localDest, s.remotePath, remote.Path), "size", remote.Size)
		}()
	}

	wg.Wait()
	return nil
}

// downloadFile downloads a single file from the FTPES server to a staging path,
// then atomically moves it to its final destination. Progress and completion are
// reported via onEvent. Returns an error on failure; the caller emits
// TransferEventFailed after all retries are exhausted.
func (s *FTPESSource) downloadFile(
	ctx context.Context,
	client ftpes.Client,
	remote ftpes.RemoteFile,
	onEvent func(TransferEvent),
) error {
	stage := stagingPath(s.localDest, remote.Path)

	// Check for a partial staging file from a previous stalled attempt and
	// resume from where it left off. The offset is only trusted when:
	//   (a) the file exists and closes cleanly (os.Stat after f.Close guarantees
	//       the OS has flushed all buffers), and
	//   (b) the offset is strictly less than the remote size — if it equals or
	//       exceeds the remote size the staging file is stale or from a
	//       different file with the same basename, so we discard it.
	var resumeOffset int64
	if fi, err := os.Stat(stage); err == nil && fi.Size() > 0 && fi.Size() < remote.Size {
		resumeOffset = fi.Size()
	}

	var f *os.File
	var err error
	if resumeOffset > 0 {
		f, err = os.OpenFile(stage, os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			resumeOffset = 0 // fall back to fresh download if we can't open
		} else {
			slog.Info("resuming partial download", "path", remote.Path, "offset", resumeOffset, "size", remote.Size)
		}
	}
	if f == nil {
		f, err = os.Create(stage)
	}
	if err != nil {
		return fmt.Errorf("open staging file: %w", err)
	}

	start := time.Now()

	pr2, pw := newPipe()
	pr := newProgressReader(pr2, remote.Size, func(bytesRead int64) {
		// bytesRead counts bytes read in this attempt only; add resumeOffset
		// to report total bytes transferred across all attempts.
		total := resumeOffset + bytesRead
		elapsed := time.Since(start).Seconds()
		var speed float64
		if elapsed > 0 {
			speed = float64(bytesRead) / elapsed
		}
		onEvent(TransferEvent{
			Kind:         TransferEventProgress,
			RemotePath:   remote.Path,
			SizeBytes:    remote.Size,
			BytesXferred: total,
			SpeedBPS:     speed,
		})
	})

	// Stall detection: derive a context that is cancelled if no bytes transfer
	// for stallTimeout. This covers both user-cancelled jobs (ctx.Done) and true
	// network stalls where the FTP server stops sending data without closing.
	stallCtx, cancelStall := context.WithCancelCause(ctx)
	defer cancelStall(nil)

	go func() {
		ticker := time.NewTicker(stallTimeout)
		defer ticker.Stop()
		var lastBytes int64
		for {
			select {
			case <-stallCtx.Done():
				return
			case <-ticker.C:
				current := pr.n.Load()
				if current == lastBytes {
					cancelStall(errTransferStalled)
					return
				}
				lastBytes = current
			}
		}
	}()

	dlDone := make(chan error, 1)
	go func() {
		// stallCtx is passed so Download closes the FTP response when stall
		// (or cancellation) is detected, unblocking the internal r.Read().
		// resumeOffset tells Download to seek the server to that position via REST.
		dlDone <- client.Download(stallCtx, remote.Path, pw, resumeOffset)
		pw.Close()
	}()

	_, copyErr := copyWithContext(stallCtx, f, pr)
	cancelStall(nil) // stop the watchdog goroutine
	if copyErr != nil {
		// Closing pr2 causes any pending pw.Write() to fail immediately,
		// which unblocks io.Copy inside Download if it hasn't exited yet.
		pr2.CloseWithError(copyErr)
	}
	dlErr := <-dlDone
	f.Close()

	var downloadErr error
	if copyErr != nil {
		// Distinguish a stall from a user/server cancellation.
		if errors.Is(context.Cause(stallCtx), errTransferStalled) {
			downloadErr = errTransferStalled
		} else {
			downloadErr = copyErr
		}
	} else if dlErr != nil {
		downloadErr = dlErr
	}

	if downloadErr != nil {
		// Do NOT remove the staging file here — a stall-caused retry will
		// pick it up and resume. Sync cleans it up after all retries are
		// exhausted or the job is cancelled.
		return downloadErr
	}

	durationMs := time.Since(start).Milliseconds()
	dst := finalPath(s.localDest, s.remotePath, remote.Path)
	if err := atomicMove(stage, dst); err != nil {
		os.Remove(stage)
		return err
	}

	mtime := remote.MTime
	onEvent(TransferEvent{
		Kind:         TransferEventDone,
		RemotePath:   remote.Path,
		SizeBytes:    remote.Size,
		BytesXferred: remote.Size,
		DurationMs:   &durationMs,
		SyncState: &db.SyncState{
			RemotePath: remote.Path,
			SizeBytes:  remote.Size,
			MTime:      &mtime,
			CopiedAt:   time.Now().UTC(),
		},
	})
	return nil
}
