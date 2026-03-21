import { useState, useEffect } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { api } from '../api/client'
import type { SyncJob, Transfer } from '../api/types'
import { JobFormModal } from '../components/JobFormModal'
import { StatusBadge } from '../components/StatusBadge'
import { ProgressBar } from '../components/ProgressBar'
import { useSSE } from '../hooks/useSSE'
import { openEventSource } from '../hooks/eventSource'
import { usePlan } from '../context/PlanContext'

function formatBytes(b: number): string {
  if (b >= 1_000_000) return `${(b / 1_000_000).toFixed(1)} MB`
  if (b >= 1_000)     return `${(b / 1_000).toFixed(1)} KB`
  return `${b} B`
}

function formatSpeed(bps: number): string {
  if (bps >= 1_000_000) return `${(bps / 1_000_000).toFixed(1)} MB/s`
  if (bps >= 1_000)     return `${(bps / 1_000).toFixed(1)} KB/s`
  return `${Math.round(bps)} B/s`
}

function TransferRow({ transfer, liveEvent }: {
  transfer: Transfer
  liveEvent?: { percent: number; speed_bps: number; bytes_xferred: number; status: string }
}) {
  const status = liveEvent?.status ?? transfer.status
  const percent = liveEvent?.percent ?? (transfer.status === 'done' ? 100 : 0)
  const speed = liveEvent?.speed_bps

  return (
    <tr className="hover:bg-gray-50 dark:hover:bg-gray-700/50">
      <td className="px-4 py-1.5 font-mono text-xs text-gray-700 dark:text-gray-300 truncate max-w-xs" title={transfer.remote_path}>
        {transfer.remote_path}
      </td>
      <td className="px-4 py-1.5 text-xs text-gray-500 dark:text-gray-400 w-24">{formatBytes(transfer.size_bytes)}</td>
      <td className="px-4 py-1.5 text-xs text-gray-400 dark:text-gray-500 text-right w-16">
        {status === 'in_progress' && speed !== undefined && speed > 0 ? formatSpeed(speed) : null}
      </td>
      <td className="px-4 py-1.5 w-48">
        {(status === 'in_progress' || status === 'done') ? (
          <ProgressBar percent={percent} />
        ) : (
          <span className="text-xs text-gray-400 dark:text-gray-500">—</span>
        )}
      </td>
      <td className="px-4 py-1.5 w-20">
        <StatusBadge status={status} />
      </td>
    </tr>
  )
}

function JobRunPreview({ jobId, triggeredAt, onDismiss }: { jobId: string; triggeredAt: number; onDismiss: () => void }) {
  const { plans, subscribePlan } = usePlan()

  // Subscribe to plan events so we can show planning progress before the run starts.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  useEffect(() => subscribePlan(jobId), [jobId])

  const planEntry = plans[jobId]

  // Poll the job's run list until a run that started at or after triggeredAt
  // appears, then poll the run detail for transfers. This prevents stale failed
  // runs from a previous trigger showing up immediately.
  const { data: runs = [] } = useQuery({
    queryKey: ['runs', jobId, 'preview'],
    queryFn: () => api.jobs.listRuns(jobId),
    refetchInterval: (q) => {
      const newRun = q.state.data?.find(r => new Date(r.started_at).getTime() >= triggeredAt)
      return (!newRun || newRun.status === 'running') ? 2000 : false
    },
  })

  const runId = runs.find(r => new Date(r.started_at).getTime() >= triggeredAt)?.id

  const { data: run } = useQuery({
    queryKey: ['run', runId],
    queryFn: () => api.runs.get(runId!),
    enabled: !!runId,
    refetchInterval: (q) => q.state.data?.status === 'running' ? 3000 : false,
  })

  const { events: liveEvents } = useSSE(run?.status === 'running' ? runId! : null)

  if (!run) {
    if (planEntry?.status === 'running') {
      return (
        <div className="border-t border-gray-100 dark:border-gray-700 px-4 py-3 flex items-center gap-2 text-xs text-gray-400 dark:text-gray-500">
          <span className="w-3 h-3 border-2 border-current border-t-transparent rounded-full animate-spin shrink-0" />
          Planning… {planEntry.scannedFiles} files, {planEntry.scannedFolders} folders scanned
        </div>
      )
    }
    if (planEntry?.status === 'done' && planEntry.result) {
      return (
        <div className="border-t border-gray-100 dark:border-gray-700 px-4 py-3 flex items-center gap-2 text-xs text-gray-400 dark:text-gray-500">
          <span className="w-3 h-3 border-2 border-current border-t-transparent rounded-full animate-spin shrink-0" />
          Run will start after plan · {planEntry.result.to_copy} to copy · {planEntry.result.to_skip} to skip
        </div>
      )
    }
    return (
      <div className="border-t border-gray-100 dark:border-gray-700 px-4 py-3 flex items-center gap-2 text-xs text-gray-400 dark:text-gray-500">
        <span className="w-3 h-3 border-2 border-current border-t-transparent rounded-full animate-spin shrink-0" />
        Starting…
      </div>
    )
  }

  const transfers = run.transfers ?? []

  return (
    <div className="border-t border-gray-100 dark:border-gray-700">
      <div className="px-4 py-2 flex items-center gap-3">
        <StatusBadge status={run.status} />
        <span className="text-xs text-gray-500 dark:text-gray-400">
          {run.total_files} total · {run.copied_files} copied · {run.skipped_files} skipped
          {run.failed_files > 0 && ` · ${run.failed_files} failed`}
        </span>
        <Link to={`/runs/${run.id}`} className="ml-auto text-xs text-blue-500 dark:text-blue-400 hover:underline">
          View details
        </Link>
        <button onClick={onDismiss} className="text-xs text-gray-400 hover:text-gray-600 dark:hover:text-gray-300">
          Dismiss
        </button>
      </div>
      {transfers.length > 0 && (
        <div className="border-t border-gray-100 dark:border-gray-700 max-h-52 overflow-y-auto">
          <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <tbody className="divide-y divide-gray-100 dark:divide-gray-700">
              {transfers.map((t) => (
                <TransferRow key={t.id} transfer={t} liveEvent={liveEvents.get(t.id)} />
              ))}
            </tbody>
          </table>
          </div>
        </div>
      )}
      {transfers.length === 0 && run.status === 'running' && (
        <div className="border-t border-gray-100 dark:border-gray-700 px-4 py-3 text-xs text-gray-400 dark:text-gray-500">
          Enumerating files…
        </div>
      )}
    </div>
  )
}

function JobRow({ job, onEdit, onDelete }: { job: SyncJob; onEdit: () => void; onDelete: () => void }) {
  const [showPreview, setShowPreview] = useState(false)
  const [triggeredAt, setTriggeredAt] = useState(0)
  const [isPlanning, setIsPlanning] = useState(false)
  const [isRunning, setIsRunning] = useState(false)
  const qc = useQueryClient()

  // Subscribe to job-level SSE events to track planning/running state.
  useEffect(() => {
    return openEventSource(`/api/jobs/${job.id}/events`, (es) => {
      es.onmessage = (e) => {
        try {
          const ev = JSON.parse(e.data) as { status: string }
          if (ev.status === 'planning') {
            setIsPlanning(true)
            setIsRunning(false)
          } else if (ev.status === 'started') {
            setIsPlanning(false)
            setIsRunning(true)
          } else if (ev.status === 'run_finished') {
            setIsPlanning(false)
            setIsRunning(false)
            qc.invalidateQueries({ queryKey: ['runs', job.id, 'preview'] })
          }
        } catch {
          // malformed event — ignore
        }
      }
    })
  }, [job.id, qc])

  const run = useMutation({
    mutationFn: () => api.jobs.planThenRun(job.id),
    onSuccess: () => {
      const now = Date.now()
      setTriggeredAt(now)
      // Invalidate the cached run list so the preview doesn't show a stale
      // failed run from before this trigger.
      qc.invalidateQueries({ queryKey: ['runs', job.id, 'preview'] })
      setShowPreview(true)
    },
  })

  const isBusy = run.isPending || isPlanning || isRunning

  function runLabel() {
    if (run.isPending || isPlanning) return 'Planning…'
    if (isRunning) return 'Running…'
    return 'Run Now'
  }

  return (
    <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg overflow-hidden">
      <div className="px-4 py-3 flex items-center gap-4">
        <div className="flex-1 min-w-0">
          <Link to={`/jobs/${job.id}`} className="font-medium text-gray-900 dark:text-gray-100 text-sm hover:text-blue-600 dark:hover:text-gray-200">
            {job.name}
          </Link>
          <p className="text-xs text-gray-500 dark:text-gray-400">{job.remote_path} → {job.local_dest}</p>
          <p className="text-xs text-gray-400 dark:text-gray-500">{job.concurrency} concurrent downloads</p>
          <p className="text-xs text-gray-400 dark:text-gray-500">
            autosync {job.enabled ? 'enabled' : 'disabled'} · every {job.interval_value} {job.interval_unit}
          </p>
        </div>
        <div className="shrink-0 flex flex-wrap gap-2">
          <button onClick={onEdit} className="btn-secondary text-xs">Edit</button>
          <button
            onClick={() => run.mutate()}
            disabled={isBusy}
            className="btn-secondary text-xs"
          >
            {runLabel()}
          </button>
          <button onClick={onDelete} className="btn-danger text-xs">Delete</button>
        </div>
      </div>
      {run.isError && (
        <p className="text-red-600 dark:text-red-400 text-xs px-4 pb-2">{(run.error as Error).message}</p>
      )}
      {showPreview && (
        <JobRunPreview jobId={job.id} triggeredAt={triggeredAt} onDismiss={() => setShowPreview(false)} />
      )}
    </div>
  )
}

export function JobsPage() {
  const qc = useQueryClient()
  const [modalOpen, setModalOpen] = useState(false)
  const [editingJob, setEditingJob] = useState<SyncJob | null>(null)

  const { data: jobs = [], isLoading } = useQuery({ queryKey: ['jobs'], queryFn: api.jobs.list })

  const remove = useMutation({
    mutationFn: api.jobs.delete,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['jobs'] }),
  })

  function openCreate() { setEditingJob(null); setModalOpen(true) }
  function openEdit(j: SyncJob) { setEditingJob(j); setModalOpen(true) }
  function closeModal() { setModalOpen(false); setEditingJob(null) }

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-xl font-semibold text-gray-900 dark:text-gray-100">Sync Jobs</h1>
        <button onClick={openCreate} className="btn-primary">Add Job</button>
      </div>

      {isLoading && <p className="text-gray-500 dark:text-gray-400 text-sm">Loading…</p>}
      {!isLoading && jobs.length === 0 && (
        <p className="text-gray-400 dark:text-gray-500 text-sm">No jobs yet. Add one to get started.</p>
      )}

      <div className="space-y-2">
        {jobs.map((j) => (
          <JobRow
            key={j.id}
            job={j}
            onEdit={() => openEdit(j)}
            onDelete={() => { if (confirm(`Delete "${j.name}"?`)) remove.mutate(j.id) }}
          />
        ))}
      </div>

      {modalOpen && (
        <JobFormModal editing={editingJob} onClose={closeModal} />
      )}
    </div>
  )
}
