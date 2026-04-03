import { useState, useEffect } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Link, useParams } from 'react-router-dom'
import { api } from '../api/client'
import { StatusBadge } from '../components/StatusBadge'
import { JobFormModal } from '../components/JobFormModal'
import { formatBytes } from '../components/RunTree'
import { RunRow } from '../components/RunRow'
import { PlanTreeView, GitPlanView } from '../components/PlanTreeView'
import type { TreeFile } from '../components/PlanTreeView'
import { usePlan } from '../context/PlanContext'
import { useLocalStorageBool } from '../hooks/useLocalStorageBool'

export function JobDetailPage() {
  const { id } = useParams<{ id: string }>()
  const qc = useQueryClient()

  const { data: job } = useQuery({ queryKey: ['jobs', id], queryFn: () => api.jobs.get(id!) })
  const { data: sources = [] } = useQuery({ queryKey: ['sources'], queryFn: api.sources.list })
  const isGitJob = !!sources.find((s) => s.id === job?.source_id && s.type === 'git')
  const { data: runs = [], isLoading } = useQuery({
    queryKey: ['runs', id],
    queryFn: () => api.jobs.listRuns(id!),
  })
  const { plans, runPlan, subscribePlan, dismissPlan, unskipFile, skipFile, subscribeJobEvents, onJobEvent } = usePlan()
  const planEntry = id ? plans[id] : undefined
  const [planOpen, setPlanOpen] = useState(true)
  const [showAllRuns, setShowAllRuns] = useState(false)
  const RUNS_LIMIT = 25

  // Auto-subscribe to plan events and job-level events via global context.
  useEffect(() => { if (!id) return; return subscribePlan(id) }, [id])  // eslint-disable-line react-hooks/exhaustive-deps
  useEffect(() => { if (!id) return; return subscribeJobEvents(id) }, [id])  // eslint-disable-line react-hooks/exhaustive-deps

  // Handle page-specific job events (plan file updates).
  useEffect(() => {
    if (!id) return
    return onJobEvent(id, (ev) => {
      const status = ev.status as string
      if (status === 'plan_file_updated') {
        if (ev.plan_action === 'skip') skipFile(id, ev.plan_path as string)
        else unskipFile(id, ev.plan_path as string)
      }
    })
  }, [id])  // eslint-disable-line react-hooks/exhaustive-deps

  const [editOpen, setEditOpen] = useState(false)
  const [hideNothingToSync, setHideNothingToSync] = useLocalStorageBool(`hideNothingToSync:${id}`, true)
  const jobIsRunning = runs[0]?.status === 'running'

  const run = useMutation({
    mutationFn: () => api.jobs.run(id!),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['runs', id] })
      if (id) dismissPlan(id)
    },
  })

  const doSkip = (f: TreeFile): Promise<void> =>
    api.jobs.skipFile(id!, f.remote_path, f.size_bytes, f.mtime)
      .then(() => { if (id) skipFile(id, f.remote_path) })
      .catch(() => {})

  const doGitSkip = (path: string, commitHash: string): Promise<void> =>
    api.jobs.skipFile(id!, path, 0, '', commitHash)
      .then(() => { if (id) skipFile(id, path) })
      .catch(() => {})

  const doUnskip = (remotePath: string): Promise<void> =>
    api.jobs.deleteFileState(id!, remotePath)
      .then(() => { if (id) unskipFile(id, remotePath) })
      .catch(() => {})

  return (
    <div>
      <div className="flex items-center gap-2 text-sm text-gray-500 dark:text-gray-400 mb-6">
        <Link to="/jobs" className="hover:text-gray-700 dark:hover:text-gray-300">Jobs</Link>
        <span>/</span>
        <span className="text-gray-900 dark:text-gray-100 font-medium">{job?.name ?? '…'}</span>
      </div>

      <div className="flex flex-wrap items-start gap-3 mb-6">
        <div className="flex-1 min-w-0">
          <h1 className="text-xl font-semibold text-gray-900 dark:text-gray-100">{job?.name}</h1>
          {job && (
            <>
              <p className="text-xs text-gray-500 dark:text-gray-400 mt-1">
                {job.git_repos?.length > 0 ? 'Repos' : job.remote_path} → {job.local_dest}
              </p>
              <p className="text-xs text-gray-400 dark:text-gray-500">{job.concurrency} concurrent downloads</p>
              <p className="text-xs text-gray-400 dark:text-gray-500">
                autosync {job.enabled ? 'enabled' : 'disabled'} · every {job.interval_value} {job.interval_unit}
              </p>
            </>
          )}
        </div>
        <div className="flex flex-wrap gap-2">
          <button
            onClick={() => setEditOpen(true)}
            disabled={planEntry?.status === 'running' || jobIsRunning}
            className="btn-secondary"
          >Edit</button>
          <button
            onClick={() => id && runPlan(id)}
            disabled={planEntry?.status === 'running' || jobIsRunning}
            className="btn-secondary"
          >
            {planEntry?.status === 'running' ? 'Planning…' : 'Plan'}
          </button>
          <button
            onClick={() => run.mutate()}
            disabled={run.isPending || planEntry?.status !== 'done' || jobIsRunning}
            title={jobIsRunning ? 'A run is already in progress' : planEntry?.status !== 'done' ? 'Plan first before running' : undefined}
            className="btn-primary"
          >
            {run.isPending ? 'Starting…' : 'Run Now'}
          </button>
        </div>
      </div>

      {run.isError && (
        <p className="text-red-600 dark:text-red-400 text-sm mb-4">{(run.error as Error).message}</p>
      )}
      {planEntry?.status === 'error' && (
        <p className="text-red-600 dark:text-red-400 text-sm mb-4">{planEntry.error}</p>
      )}

      {planEntry && planEntry.status !== 'error' && (
        <div className="mb-8 card overflow-hidden">
          <div className="flex flex-wrap items-center gap-4 px-4 py-3">
            <button
              onClick={() => setPlanOpen((o) => !o)}
              className="flex flex-wrap items-center gap-4 flex-1 min-w-0 hover:opacity-80 transition-opacity text-left"
            >
              <StatusBadge status={planEntry.status === 'running' ? 'planning' : 'plan'} />
              <div className="flex-1 min-w-0">
                {planEntry.status === 'running' && (planEntry.scannedFiles > 0 || planEntry.scannedFolders > 0) && (
                  <p className="text-xs text-gray-500 dark:text-gray-400">
                    {planEntry.scannedFiles} files, {planEntry.scannedFolders} folders found
                  </p>
                )}
              </div>
              {planEntry.result && (() => {
                const planSize = planEntry.result.files.filter(f => f.action === 'copy').reduce((s, f) => s + f.size_bytes, 0)
                return (
                  <div className="flex flex-wrap gap-4 text-xs text-gray-500 dark:text-gray-400">
                    {planSize > 0 && <span>{formatBytes(planSize)}</span>}
                    <span>{planEntry.result.files.length} total</span>
                    <span className="text-green-600 dark:text-green-400">{planEntry.result.to_copy} to copy</span>
                    <span className="text-yellow-600 dark:text-yellow-400">{planEntry.result.to_skip} to skip</span>
                  </div>
                )
              })()}
              <span className="text-gray-400 dark:text-gray-500 text-xs w-3 shrink-0">{planOpen ? '▾' : '▸'}</span>
            </button>
            {planEntry.status !== 'running' && (
              <button onClick={() => id && dismissPlan(id)} className="text-xs text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 shrink-0">
                Dismiss
              </button>
            )}
          </div>
          {planOpen && (
            planEntry.status === 'running' ? (
              <div className="border-t border-gray-100 dark:border-gray-700 px-4 py-8 flex items-center justify-center gap-3 text-sm text-gray-400 dark:text-gray-500">
                <span className="w-4 h-4 border-2 border-current border-t-transparent rounded-full animate-spin shrink-0" />
                Scanning…
                {(planEntry.scannedFiles > 0 || planEntry.scannedFolders > 0) && (
                  <span>{planEntry.scannedFiles} files, {planEntry.scannedFolders} folders found</span>
                )}
              </div>
            ) : planEntry.result && (
              <div className="border-t border-gray-100 dark:border-gray-700">
                {isGitJob
                  ? <GitPlanView files={planEntry.result.files} onSkip={doGitSkip} onUnskip={doUnskip} />
                  : <PlanTreeView files={planEntry.result.files} remotePath={job?.remote_path ?? ''} onSkip={doSkip} onUnskip={doUnskip} />
                }
              </div>
            )
          )}
        </div>
      )}

      <div className="flex items-center gap-3 mb-3">
        <h2 className="text-sm font-medium text-gray-700 dark:text-gray-300">Run History</h2>
        <label className="flex items-center gap-2 cursor-pointer">
          <button
            role="switch"
            aria-checked={hideNothingToSync}
            onClick={() => setHideNothingToSync((v) => !v)}
            className={`relative inline-flex h-5 w-9 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors ${hideNothingToSync ? 'bg-blue-600' : 'bg-gray-300 dark:bg-gray-600'}`}
          >
            <span className={`inline-block h-4 w-4 rounded-full bg-white shadow transition-transform ${hideNothingToSync ? 'translate-x-4' : 'translate-x-0'}`} />
          </button>
          <span className="text-xs text-gray-500 dark:text-gray-400">Hide "Nothing to Sync"</span>
        </label>
        {runs.length > RUNS_LIMIT && (
          <button
            onClick={() => setShowAllRuns((v) => !v)}
            className="text-xs text-gray-400 dark:text-gray-500 hover:text-gray-600 dark:hover:text-gray-300"
          >
            {showAllRuns ? 'Show less' : `Show all ${runs.length}`}
          </button>
        )}
      </div>

      {isLoading && <p className="text-gray-500 dark:text-gray-400 text-sm">Loading…</p>}
      {!isLoading && runs.length === 0 && (
        <p className="text-gray-400 dark:text-gray-500 text-sm">No runs yet.</p>
      )}

      <div className="space-y-2">
        {(() => {
          const filtered = hideNothingToSync ? runs.filter((r) => r.status !== 'nothing_to_sync') : runs
          return (showAllRuns ? filtered : filtered.slice(0, RUNS_LIMIT)).map((run) => (
            <RunRow key={run.id} run={run} remotePath={job?.remote_path ?? ''} jobId={id!} isGit={isGitJob} />
          ))
        })()}
      </div>

      {editOpen && job && (
        <JobFormModal editing={job} onClose={() => setEditOpen(false)} />
      )}
    </div>
  )
}
