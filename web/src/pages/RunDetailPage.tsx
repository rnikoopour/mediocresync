import { useQuery } from '@tanstack/react-query'
import { Link, useParams } from 'react-router-dom'
import { api } from '../api/client'
import { StatusBadge } from '../components/StatusBadge'
import { RunTreeView, formatBytes, formatSpeed } from '../components/RunTree'
import { useRunState } from '../hooks/useRunState'
import { useLocalStorageBool } from '../hooks/useLocalStorageBool'
import { formatDateTime } from '../utils/time'
import { formatDuration } from '../utils/format'

export function RunDetailPage() {
  const { id } = useParams<{ id: string }>()
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

  const isActive = run?.status === 'running' || run?.status === 'canceling'
  const { liveEvents, runEnded, liveSpeedBps, avgSpeedBps } = useRunState(isActive ? id! : null, run?.job_id ?? '', run)

  if (isLoading) return <p className="text-gray-500 dark:text-gray-400 text-sm">Loading…</p>
  if (!run) return <p className="text-red-500 text-sm">Run not found.</p>
  const transfers = run.transfers ?? []

  const duration = run.finished_at
    ? formatDuration(new Date(run.finished_at).getTime() - new Date(run.started_at).getTime())
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

