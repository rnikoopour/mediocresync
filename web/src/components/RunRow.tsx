import { useState, useEffect } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { api } from '../api/client'
import type { Run } from '../api/types'
import { StatusBadge } from './StatusBadge'
import { RunTreeView, RunTabBar, formatBytes, formatSpeed } from './RunTree'
import type { RunTab } from './RunTree'
import { useSSE } from '../hooks/useSSE'
import { useLocalStorageBool } from '../hooks/useLocalStorageBool'
import { formatDateTime } from '../utils/time'

export function formatDuration(ms: number): string {
  const s = Math.floor(ms / 1000)
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  const sec = s % 60
  if (h > 0) return `${h}h ${m}m ${sec}s`
  if (m > 0) return `${m}m ${sec}s`
  return `${sec}s`
}

export function useElapsed(startedAt: string, isRunning: boolean): string {
  const [now, setNow] = useState(() => Date.now())
  useEffect(() => {
    if (!isRunning) return
    const id = setInterval(() => setNow(Date.now()), 1000)
    return () => clearInterval(id)
  }, [isRunning])
  return formatDuration(now - new Date(startedAt).getTime())
}

function GitRunView({ transfers, isRunning }: { transfers: import('../api/types').Transfer[]; isRunning: boolean }) {
  const [tab, setTab] = useState<RunTab>('planned')
  const filtered = tab === 'all' ? transfers : transfers.filter((t) => {
    const status = !isRunning && t.status === 'pending' ? 'not_copied' : t.status
    if (tab === 'planned')     return status !== 'skipped'
    if (tab === 'in_progress') return status === 'in_progress' || status === 'pending'
    if (tab === 'copied')      return status === 'done'
    if (tab === 'not_copied')  return status === 'not_copied' || status === 'failed' || status === 'canceled'
    return true
  })
  return (
    <div className="border-t border-gray-100 dark:border-gray-700">
      <RunTabBar tab={tab} onTab={setTab} isRunning={isRunning} />
      <div className="divide-y divide-gray-100 dark:divide-gray-700 py-1">
        {filtered.length === 0
          ? <p className="px-4 py-4 text-xs text-center text-gray-400 dark:text-gray-500">No repos match this filter.</p>
          : filtered.map((t) => (
            <div key={t.id} className="px-4 py-2">
              <div className="flex items-center gap-3">
                <StatusBadge status={t.status} />
                <span className="font-mono text-xs text-gray-700 dark:text-gray-300 flex-1 min-w-0 break-all">{t.remote_path}</span>
                {t.error_msg && <span className="text-xs text-red-500 dark:text-red-400 truncate">{t.error_msg}</span>}
              </div>
              {(t.previous_commit_hash || t.current_commit_hash) && (
                <div className="font-mono text-xs text-gray-400 dark:text-gray-500 mt-0.5 ml-[calc(1.5rem+0.75rem)]">
                  {t.previous_commit_hash
                    ? <>{t.previous_commit_hash.slice(0, 7)} → {t.current_commit_hash?.slice(0, 7)}</>
                    : <>new → {t.current_commit_hash?.slice(0, 7)}</>
                  }
                </div>
              )}
            </div>
          ))
        }
      </div>
    </div>
  )
}

export function RunRow({ run: initialRun, remotePath, jobId, isGit }: { run: Run; remotePath: string; jobId: string; isGit: boolean }) {
  const qc = useQueryClient()
  const [use24h] = useLocalStorageBool('use24hTime', false)
  const [open, setOpen] = useState(initialRun.status === 'running')

  const { data: fetchedRun } = useQuery({
    queryKey: ['run', initialRun.id],
    queryFn: () => api.runs.get(initialRun.id),
    enabled: open,
  })
  const run = fetchedRun ?? initialRun
  const isDetailLoaded = fetchedRun !== undefined

  const [cancelling, setCancelling] = useState(false)
  const cancel = useMutation({
    mutationFn: () => api.jobs.cancel(jobId),
    onSuccess: () => setCancelling(true),
  })

  const { events: liveEvents, runStatus, isDone } = useSSE(open ? run.id : null)

  // When a transfer completes, re-fetch the run to update the copied count.
  useEffect(() => {
    const hasDone = Array.from(liveEvents.values()).some((e) => e.status === 'done')
    if (hasDone) qc.invalidateQueries({ queryKey: ['run', initialRun.id] })
  }, [liveEvents, initialRun.id, qc])

  // When the run reaches a terminal status, re-fetch to get finished_at,
  // bytes_copied, and transfers_started_at for the final avg speed display.
  // Also fires when isDone is set — handles the case where the run_status event
  // was dropped (non-blocking broker buffer overflow) but done was still received.
  const terminalStatuses = ['completed', 'failed', 'partial', 'canceled', 'server_stopped', 'nothing_to_sync']
  useEffect(() => {
    const isTerminalStatus = runStatus && terminalStatuses.includes(runStatus)
    if (isTerminalStatus || isDone) {
      qc.invalidateQueries({ queryKey: ['run', initialRun.id] })
      qc.invalidateQueries({ queryKey: ['runs', initialRun.job_id] })
    }
  }, [runStatus, isDone, initialRun.id, initialRun.job_id, qc])
  const effectiveStatus = (runStatus && runStatus !== 'canceling') ? runStatus : run.status
  const isRunning = effectiveStatus === 'running' || effectiveStatus === 'canceling'
  const isCancelling = cancelling || runStatus === 'canceling' || run.status === 'canceling'
  const elapsed = useElapsed(run.started_at, isRunning)

  if (cancelling && !isRunning && runStatus !== 'canceling') setCancelling(false)

  const transfers = run.transfers

  const duration = run.finished_at
    ? formatDuration(new Date(run.finished_at).getTime() - new Date(run.started_at).getTime())
    : isRunning ? elapsed : null

  const liveSpeedBps = isRunning
    ? Array.from(liveEvents.values()).reduce((s, e) => e.status === 'in_progress' ? s + e.speed_bps : s, 0)
    : 0

  const avgSpeedBps = !isRunning && run.finished_at && run.bytes_copied > 0 && run.transfers_started_at
    ? run.bytes_copied / ((new Date(run.finished_at).getTime() - new Date(run.transfers_started_at).getTime()) / 1000)
    : null

  const pendingFiles = run.total_files - run.copied_files - run.skipped_files - run.failed_files
  const hasSpeedOrSize = run.total_size_bytes > 0 || run.bytes_copied > 0 || liveSpeedBps > 0 || avgSpeedBps !== null

  return (
    <div className="card overflow-hidden">
      <div className="flex items-center gap-2 px-4 py-3">
        <button
          onClick={() => setOpen((o) => !o)}
          className="flex items-center gap-3 flex-1 min-w-0 hover:opacity-80 transition-opacity text-left"
        >
          <div className="shrink-0"><StatusBadge status={effectiveStatus} /></div>
          <div className="flex-1 min-w-0 space-y-0.5">
            <p className="text-xs text-gray-500 dark:text-gray-400">
              Started {formatDateTime(run.started_at, use24h)}{duration && ` · ${duration}`}
            </p>
            <div className="flex flex-wrap gap-x-3 gap-y-0 text-xs text-gray-500 dark:text-gray-400">
              <span>{run.total_files} total</span>
              <span className="text-green-600 dark:text-green-400">{run.copied_files} copied</span>
              <span className="text-yellow-600 dark:text-yellow-400">{run.skipped_files} skipped</span>
              {run.failed_files > 0 && <span className="text-red-600 dark:text-red-400">{run.failed_files} failed</span>}
              {pendingFiles > 0 && <span>{isRunning ? `${pendingFiles} pending` : `${pendingFiles} not copied`}</span>}
            </div>
            {hasSpeedOrSize && (
              <div className="flex flex-wrap gap-x-3 gap-y-0 text-xs text-gray-500 dark:text-gray-400">
                {(run.bytes_copied > 0 || run.total_size_bytes > 0) && (
                  <span>{formatBytes(isRunning ? run.total_size_bytes : (run.bytes_copied || run.total_size_bytes))}</span>
                )}
                {liveSpeedBps > 0 && <span className="text-blue-600 dark:text-blue-400">{formatSpeed(liveSpeedBps)}</span>}
                {avgSpeedBps !== null && <span>avg {formatSpeed(avgSpeedBps)}</span>}
              </div>
            )}
          </div>
          <span className="text-gray-400 dark:text-gray-500 text-xs shrink-0">{open ? '▾' : '▸'}</span>
        </button>
        <div className="flex items-center gap-2 shrink-0">
          <Link
            to={`/runs/${run.id}`}
            onClick={(e) => e.stopPropagation()}
            className="text-gray-400 dark:text-gray-500 hover:text-gray-600 dark:hover:text-gray-300 text-xs"
            title="View run details"
          >
            ↗
          </Link>
          {(effectiveStatus === 'running' || isCancelling) && (
            <button
              onClick={() => cancel.mutate()}
              disabled={isCancelling}
              className="btn-danger text-xs"
            >
              {isCancelling ? 'Cancelling…' : 'Cancel'}
            </button>
          )}
        </div>
      </div>

      {open && (
        !isDetailLoaded ? (
          <p className="border-t border-gray-100 dark:border-gray-700 px-4 py-4 text-xs text-center text-gray-400 dark:text-gray-500">
            Loading…
          </p>
        ) : !transfers?.length ? (
          <div className="border-t border-gray-100 dark:border-gray-700 px-4 py-4 text-xs text-center text-gray-400 dark:text-gray-500">
            {run.error_msg
              ? <p className="text-red-500 dark:text-red-400 font-mono break-all">{run.error_msg}</p>
              : <p>{isGit ? 'No repos to sync.' : 'No transfers recorded.'}</p>
            }
          </div>
        ) : isGit ? (
          <GitRunView transfers={transfers} isRunning={isRunning} />
        ) : (
          <RunTreeView transfers={transfers} remotePath={remotePath} liveEvents={liveEvents} runEnded={!isRunning} />
        )
      )}
    </div>
  )
}
