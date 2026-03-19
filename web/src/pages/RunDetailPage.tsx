import { useQuery } from '@tanstack/react-query'
import { Link, useParams } from 'react-router-dom'
import { api } from '../api/client'
import type { Transfer } from '../api/types'
import { StatusBadge } from '../components/StatusBadge'
import { ProgressBar } from '../components/ProgressBar'
import { useSSE } from '../hooks/useSSE'

function formatBytes(b: number): string {
  if (b >= 1_000_000) return `${(b / 1_000_000).toFixed(1)} MB`
  if (b >= 1_000)     return `${(b / 1_000).toFixed(1)} KB`
  return `${b} B`
}

export function RunDetailPage() {
  const { id } = useParams<{ id: string }>()

  const { data: run, isLoading } = useQuery({
    queryKey: ['run', id],
    queryFn: () => api.runs.get(id!),
    // Poll while running so counts update even without SSE
    refetchInterval: (query) => query.state.data?.status === 'running' ? 3000 : false,
  })

  // Only subscribe to SSE while the run is active
  const { events: liveEvents } = useSSE(run?.status === 'running' ? id! : null)

  if (isLoading) return <p className="text-gray-500 dark:text-gray-400 text-sm">Loading…</p>
  if (!run) return <p className="text-red-500 text-sm">Run not found.</p>

  return (
    <div>
      <div className="flex items-center gap-2 text-sm text-gray-500 dark:text-gray-400 mb-6">
        <Link to="/jobs" className="hover:text-gray-700 dark:hover:text-gray-300">Jobs</Link>
        <span>/</span>
        <Link to={`/jobs/${run.job_id}`} className="hover:text-gray-700 dark:hover:text-gray-300">{run.job_id}</Link>
        <span>/</span>
        <span className="text-gray-900 dark:text-gray-100 font-medium">Run</span>
      </div>

      <div className="flex items-center gap-4 mb-6">
        <StatusBadge status={run.status} />
        <p className="text-sm text-gray-500 dark:text-gray-400">
          Started {new Date(run.started_at).toLocaleString()}
          {run.finished_at && ` · Finished ${new Date(run.finished_at).toLocaleString()}`}
        </p>
        <div className="ml-auto flex gap-4 text-sm text-gray-600 dark:text-gray-400">
          <span>{run.total_files} total</span>
          <span className="text-green-600 dark:text-green-400">{run.copied_files} copied</span>
          <span className="text-yellow-600 dark:text-yellow-400">{run.skipped_files} skipped</span>
          {run.failed_files > 0 && <span className="text-red-600 dark:text-red-400">{run.failed_files} failed</span>}
        </div>
      </div>

      <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg overflow-hidden">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-700/50">
              <th className="text-left px-4 py-2 font-medium text-gray-600 dark:text-gray-400 text-xs">File</th>
              <th className="text-left px-4 py-2 font-medium text-gray-600 dark:text-gray-400 text-xs w-32">Size</th>
              <th className="text-left px-4 py-2 font-medium text-gray-600 dark:text-gray-400 text-xs w-56">Progress</th>
              <th className="text-left px-4 py-2 font-medium text-gray-600 dark:text-gray-400 text-xs w-24">Status</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-100 dark:divide-gray-700">
            {(run.transfers ?? []).map((t) => (
              <TransferRow key={t.id} transfer={t} liveEvent={liveEvents.get(t.id)} />
            ))}
            {(run.transfers ?? []).length === 0 && (
              <tr>
                <td colSpan={4} className="px-4 py-6 text-center text-gray-400 dark:text-gray-500 text-xs">
                  {run.status === 'running' ? 'Enumerating files…' : 'No transfers recorded.'}
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}

function TransferRow({ transfer, liveEvent }: {
  transfer: Transfer
  liveEvent?: { percent: number; speed_bps: number; bytes_xferred: number; status: string } | undefined
}) {
  const status = liveEvent?.status ?? transfer.status
  const percent = liveEvent?.percent ?? (transfer.status === 'done' ? 100 : 0)
  const speed = liveEvent?.speed_bps

  return (
    <tr className="hover:bg-gray-50 dark:hover:bg-gray-700/50">
      <td className="px-4 py-2 font-mono text-xs text-gray-700 dark:text-gray-300 truncate max-w-xs" title={transfer.remote_path}>
        {transfer.remote_path}
      </td>
      <td className="px-4 py-2 text-xs text-gray-500 dark:text-gray-400">{formatBytes(transfer.size_bytes)}</td>
      <td className="px-4 py-2">
        {(status === 'in_progress' || status === 'done') ? (
          <ProgressBar percent={percent} speedBps={status === 'in_progress' ? speed : undefined} />
        ) : (
          <span className="text-xs text-gray-400 dark:text-gray-500">—</span>
        )}
      </td>
      <td className="px-4 py-2">
        <StatusBadge status={status} />
        {transfer.error_msg && (
          <p className="text-xs text-red-500 mt-0.5 truncate" title={transfer.error_msg}>{transfer.error_msg}</p>
        )}
      </td>
    </tr>
  )
}
