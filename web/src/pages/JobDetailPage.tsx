import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Link, useParams } from 'react-router-dom'
import { api } from '../api/client'
import type { PlanResult } from '../api/types'
import { StatusBadge } from '../components/StatusBadge'

function formatBytes(b: number): string {
  if (b >= 1_000_000) return `${(b / 1_000_000).toFixed(1)} MB`
  if (b >= 1_000)     return `${(b / 1_000).toFixed(1)} KB`
  return `${b} B`
}

export function JobDetailPage() {
  const { id } = useParams<{ id: string }>()
  const qc = useQueryClient()
  const [plan, setPlan] = useState<PlanResult | null>(null)

  const { data: job } = useQuery({ queryKey: ['jobs', id], queryFn: () => api.jobs.get(id!) })
  const { data: runs = [], isLoading } = useQuery({ queryKey: ['runs', id], queryFn: () => api.jobs.listRuns(id!) })

  const trigger = useMutation({
    mutationFn: () => api.jobs.trigger(id!),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['runs', id] }),
  })

  const planMutation = useMutation({
    mutationFn: () => api.jobs.plan(id!),
    onSuccess: (result) => setPlan(result),
  })

  return (
    <div>
      <div className="flex items-center gap-2 text-sm text-gray-500 mb-6">
        <Link to="/jobs" className="hover:text-gray-700">Jobs</Link>
        <span>/</span>
        <span className="text-gray-900 font-medium">{job?.name ?? '…'}</span>
      </div>

      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-xl font-semibold text-gray-900">{job?.name}</h1>
          {job && (
            <p className="text-xs text-gray-500 mt-1">
              {job.remote_path} → {job.local_dest} · every {job.interval_value} {job.interval_unit}
            </p>
          )}
        </div>
        <div className="flex gap-2">
          <button
            onClick={() => { setPlan(null); planMutation.mutate() }}
            disabled={planMutation.isPending}
            className="btn-secondary"
          >
            {planMutation.isPending ? 'Planning…' : 'Plan'}
          </button>
          <button
            onClick={() => trigger.mutate()}
            disabled={trigger.isPending}
            className="btn-primary"
          >
            {trigger.isPending ? 'Starting…' : 'Run Now'}
          </button>
        </div>
      </div>

      {trigger.isError && (
        <p className="text-red-600 text-sm mb-4">{(trigger.error as Error).message}</p>
      )}
      {planMutation.isError && (
        <p className="text-red-600 text-sm mb-4">{(planMutation.error as Error).message}</p>
      )}

      {plan && (
        <div className="mb-8">
          <div className="flex items-center justify-between mb-3">
            <h2 className="text-sm font-medium text-gray-700">
              Plan Result
              <span className="ml-2 font-normal text-gray-400">
                {plan.to_copy} to copy · {plan.to_skip} to skip · {plan.files.length} total
              </span>
            </h2>
            <button onClick={() => setPlan(null)} className="text-xs text-gray-400 hover:text-gray-600">
              Dismiss
            </button>
          </div>
          <div className="bg-white border border-gray-200 rounded-lg overflow-hidden">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-gray-200 bg-gray-50">
                  <th className="text-left px-4 py-2 font-medium text-gray-600 text-xs">File</th>
                  <th className="text-left px-4 py-2 font-medium text-gray-600 text-xs w-28">Size</th>
                  <th className="text-left px-4 py-2 font-medium text-gray-600 text-xs w-24">Action</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-100">
                {plan.files.map((f) => (
                  <tr key={f.remote_path} className="hover:bg-gray-50">
                    <td className="px-4 py-2 font-mono text-xs text-gray-700 truncate max-w-xs" title={f.remote_path}>
                      {f.remote_path}
                    </td>
                    <td className="px-4 py-2 text-xs text-gray-500">{formatBytes(f.size_bytes)}</td>
                    <td className="px-4 py-2">
                      <StatusBadge status={f.action === 'copy' ? 'pending' : 'skipped'} />
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      <h2 className="text-sm font-medium text-gray-700 mb-3">Run History</h2>

      {isLoading && <p className="text-gray-500 text-sm">Loading…</p>}
      {!isLoading && runs.length === 0 && (
        <p className="text-gray-400 text-sm">No runs yet.</p>
      )}

      <div className="space-y-2">
        {runs.map((run) => (
          <Link
            key={run.id}
            to={`/runs/${run.id}`}
            className="block bg-white border border-gray-200 rounded-lg px-4 py-3 hover:border-blue-300 transition-colors"
          >
            <div className="flex items-center gap-4">
              <StatusBadge status={run.status} />
              <div className="flex-1 min-w-0">
                <p className="text-xs text-gray-500">
                  Started {new Date(run.started_at).toLocaleString()}
                  {run.finished_at && ` · Finished ${new Date(run.finished_at).toLocaleString()}`}
                </p>
              </div>
              <div className="flex gap-4 text-xs text-gray-500">
                <span>{run.total_files} total</span>
                <span className="text-green-600">{run.copied_files} copied</span>
                <span className="text-yellow-600">{run.skipped_files} skipped</span>
                {run.failed_files > 0 && <span className="text-red-600">{run.failed_files} failed</span>}
              </div>
            </div>
          </Link>
        ))}
      </div>
    </div>
  )
}
