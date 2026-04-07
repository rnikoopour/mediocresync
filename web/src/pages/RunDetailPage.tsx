import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect } from 'react'
import { Link, useParams } from 'react-router-dom'
import { api } from '../api/client'
import { StatusBadge } from '../components/StatusBadge'
import { RunTreeView, formatBytes, formatSpeed } from '../components/RunTree'
import { useSSE } from '../hooks/useSSE'
import { useLocalStorageBool } from '../hooks/useLocalStorageBool'
import { formatDateTime } from '../utils/time'

export function RunDetailPage() {
  const { id } = useParams<{ id: string }>()
  const qc = useQueryClient()
  const [use24h] = useLocalStorageBool('use24hTime', false)

  const { data: run, isLoading } = useQuery({
    queryKey: ['run', id],
    queryFn: () => api.runs.get(id!),
  })

  const { data: job } = useQuery({
    queryKey: ['job', run?.job_id],
    queryFn: () => api.jobs.get(run!.job_id),
    enabled: !!run,
  })

  const { events: liveEvents, runStatus } = useSSE(run?.status === 'running' || run?.status === 'canceling' ? id! : null)

  // When the run finishes (SSE run_status fires), fetch the final state.
  useEffect(() => {
    if (runStatus) qc.invalidateQueries({ queryKey: ['run', id] })
  }, [runStatus, id, qc])

  if (isLoading) return <p className="text-gray-500 dark:text-gray-400 text-sm">Loading…</p>
  if (!run) return <p className="text-red-500 text-sm">Run not found.</p>

  const runEnded = run.status !== 'running' && run.status !== 'canceling'
  const transfers = run.transfers ?? []

  const duration = run.finished_at
    ? formatDuration(new Date(run.finished_at).getTime() - new Date(run.started_at).getTime())
    : null

  const liveSpeedBps = !runEnded
    ? Array.from(liveEvents.values()).reduce((s, e) => e.status === 'in_progress' ? s + e.speed_bps : s, 0)
    : 0

  const avgSpeedBps = runEnded && run.finished_at && run.total_size_bytes > 0
    ? run.total_size_bytes / ((new Date(run.finished_at).getTime() - new Date(run.started_at).getTime()) / 1000)
    : null

  return (
    <div>
      <div className="flex items-center gap-2 text-sm text-gray-500 dark:text-gray-400 mb-6">
        <Link to="/jobs" className="hover:text-gray-700 dark:hover:text-gray-300">Jobs</Link>
        <span>/</span>
        <Link to={`/jobs/${run.job_id}`} className="hover:text-gray-700 dark:hover:text-gray-300">{job?.name ?? run.job_id}</Link>
        <span>/</span>
        <span className="text-gray-900 dark:text-gray-100 font-medium">{formatDateTime(run.started_at, use24h)}</span>
      </div>

      <div className="card overflow-hidden mb-6">
        <div className="flex flex-wrap items-center gap-4 px-4 py-3">
          <StatusBadge status={run.status} />
          <div className="flex-1 min-w-0">
            <p className="text-xs text-gray-500 dark:text-gray-400">
              Started {formatDateTime(run.started_at, use24h)}
              {duration && ` · ${duration}`}
            </p>
          </div>
          <div className="flex flex-wrap gap-4 text-xs text-gray-500 dark:text-gray-400">
            {run.total_size_bytes > 0 && <span>{formatBytes(run.total_size_bytes)}</span>}
            {liveSpeedBps > 0 && <span className="text-blue-600 dark:text-blue-400">{formatSpeed(liveSpeedBps)}</span>}
            {avgSpeedBps !== null && <span>avg {formatSpeed(avgSpeedBps)}</span>}
            <span>{run.total_files} total</span>
            <span className="text-green-600 dark:text-green-400">{run.copied_files} copied</span>
            <span className="text-yellow-600 dark:text-yellow-400">{run.skipped_files} skipped</span>
            {run.failed_files > 0 && <span className="text-red-600 dark:text-red-400">{run.failed_files} failed</span>}
          </div>
        </div>

        {run.error_msg && (
          <div className="border-t border-gray-100 dark:border-gray-700 px-4 py-3">
            <p className="text-xs text-red-500 dark:text-red-400 font-mono break-all">{run.error_msg}</p>
          </div>
        )}

        {transfers.length > 0 && (
          <RunTreeView
            transfers={transfers}
            remotePath={job?.remote_path ?? ''}
            liveEvents={liveEvents}
            runEnded={runEnded}
            scrollable={false}
          />
        )}
        {transfers.length === 0 && !run.error_msg && (
          <p className="border-t border-gray-100 dark:border-gray-700 px-4 py-4 text-xs text-center text-gray-400 dark:text-gray-500">
            {run.status === 'running' ? 'Enumerating files…' : 'No transfers recorded.'}
          </p>
        )}
      </div>
    </div>
  )
}

function formatDuration(ms: number): string {
  const s = Math.floor(ms / 1000)
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  const sec = s % 60
  if (h > 0) return `${h}h ${m}m ${sec}s`
  if (m > 0) return `${m}m ${sec}s`
  return `${sec}s`
}
