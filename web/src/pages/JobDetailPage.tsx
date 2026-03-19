import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Link, useParams } from 'react-router-dom'
import { api } from '../api/client'
import type { PlanFile } from '../api/types'
import { StatusBadge } from '../components/StatusBadge'
import { JobFormModal } from '../components/JobFormModal'
import { usePlan } from '../context/PlanContext'

function formatBytes(b: number): string {
  if (b >= 1_000_000) return `${(b / 1_000_000).toFixed(1)} MB`
  if (b >= 1_000)     return `${(b / 1_000).toFixed(1)} KB`
  return `${b} B`
}

// ── Plan tree ────────────────────────────────────────────────────────────────

type TreeFile = { type: 'file'; name: string; size_bytes: number; action: 'copy' | 'skip' }
type TreeFolder = { type: 'folder'; name: string; children: TreeNode[] }
type TreeNode = TreeFile | TreeFolder

function buildTree(files: PlanFile[], remotePath: string): TreeNode[] {
  const base = remotePath.replace(/\/+$/, '')
  const root: TreeFolder = { type: 'folder', name: '', children: [] }

  for (const file of files) {
    let rel = file.remote_path.startsWith(base + '/')
      ? file.remote_path.slice(base.length + 1)
      : file.remote_path

    const segments = rel.split('/').filter(Boolean)
    if (segments.length === 0) continue

    let cur = root
    for (let i = 0; i < segments.length - 1; i++) {
      const seg = segments[i]
      let child = cur.children.find((c): c is TreeFolder => c.type === 'folder' && c.name === seg)
      if (!child) {
        child = { type: 'folder', name: seg, children: [] }
        cur.children.push(child)
      }
      cur = child
    }
    cur.children.push({ type: 'file', name: segments[segments.length - 1], size_bytes: file.size_bytes, action: file.action })
  }

  return root.children
}

function FolderNode({ node, depth }: { node: TreeFolder; depth: number }) {
  const [open, setOpen] = useState(true)
  const indent = depth * 16

  return (
    <div>
      <button
        onClick={() => setOpen(!open)}
        className="w-full flex items-center gap-2 py-1.5 hover:bg-blue-50 dark:hover:bg-gray-700/60 text-left"
        style={{ paddingLeft: `${16 + indent}px`, paddingRight: '16px' }}
      >
        <span className="text-blue-400 text-xs w-3 shrink-0">{open ? '▾' : '▸'}</span>
        <span className="text-blue-500 shrink-0">📁</span>
        <span className="font-mono text-xs font-semibold text-blue-700 dark:text-gray-200">{node.name}</span>
      </button>
      {open && (
        <div className="border-l border-blue-100 dark:border-gray-600" style={{ marginLeft: `${16 + indent + 12}px` }}>
          {node.children.map((child, i) =>
            child.type === 'folder'
              ? <FolderNode key={child.name + i} node={child} depth={depth + 1} />
              : <FileRow key={child.name + i} node={child} depth={depth + 1} />
          )}
        </div>
      )}
    </div>
  )
}

function FileRow({ node }: { node: TreeFile; depth: number }) {
  return (
    <div
      className="flex items-center gap-4 py-1 hover:bg-gray-50 dark:hover:bg-gray-700/50"
      style={{ paddingLeft: '12px', paddingRight: '16px' }}
    >
      <span className="text-gray-400 dark:text-gray-500 shrink-0">📄</span>
      <span className="font-mono text-xs text-gray-600 dark:text-gray-300 flex-1 truncate">{node.name}</span>
      <span className="text-xs text-gray-400 dark:text-gray-500 shrink-0">{formatBytes(node.size_bytes)}</span>
      <span className="shrink-0"><StatusBadge status={node.action === 'copy' ? 'pending' : 'skipped'} /></span>
    </div>
  )
}

function PlanTreeView({ files, remotePath }: { files: PlanFile[]; remotePath: string }) {
  const nodes = buildTree(files, remotePath)
  return (
    <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg overflow-hidden py-1">
      {nodes.map((n, i) =>
        n.type === 'folder'
          ? <FolderNode key={n.name + i} node={n} depth={0} />
          : <FileRow key={n.name + i} node={n} depth={0} />
      )}
    </div>
  )
}

export function JobDetailPage() {
  const { id } = useParams<{ id: string }>()
  const qc = useQueryClient()

  const { data: job } = useQuery({ queryKey: ['jobs', id], queryFn: () => api.jobs.get(id!) })
  const { data: runs = [], isLoading } = useQuery({ queryKey: ['runs', id], queryFn: () => api.jobs.listRuns(id!) })
  const { plans, runPlan, dismissPlan } = usePlan()
  const planEntry = id ? plans[id] : undefined
  const [editOpen, setEditOpen] = useState(false)

  const trigger = useMutation({
    mutationFn: () => api.jobs.trigger(id!),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['runs', id] }),
  })

  return (
    <div>
      <div className="flex items-center gap-2 text-sm text-gray-500 dark:text-gray-400 mb-6">
        <Link to="/jobs" className="hover:text-gray-700 dark:hover:text-gray-300">Jobs</Link>
        <span>/</span>
        <span className="text-gray-900 dark:text-gray-100 font-medium">{job?.name ?? '…'}</span>
      </div>

      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-xl font-semibold text-gray-900 dark:text-gray-100">{job?.name}</h1>
          {job && (
            <p className="text-xs text-gray-500 dark:text-gray-400 mt-1">
              {job.remote_path} → {job.local_dest} · every {job.interval_value} {job.interval_unit}
            </p>
          )}
        </div>
        <div className="flex gap-2">
          <button onClick={() => setEditOpen(true)} className="btn-secondary">Edit</button>
          <button
            onClick={() => id && runPlan(id)}
            disabled={planEntry?.status === 'running'}
            className="btn-secondary"
          >
            {planEntry?.status === 'running' ? 'Planning…' : 'Plan'}
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
        <p className="text-red-600 dark:text-red-400 text-sm mb-4">{(trigger.error as Error).message}</p>
      )}
      {planEntry?.status === 'error' && (
        <p className="text-red-600 dark:text-red-400 text-sm mb-4">{planEntry.error}</p>
      )}

      {planEntry && planEntry.status !== 'error' && (
        <div className="mb-8">
          <div className="flex items-center justify-between mb-3">
            <h2 className="text-sm font-medium text-gray-700 dark:text-gray-300">
              Plan Result
              {planEntry.status === 'running' ? (
                <span className="ml-2 font-normal text-gray-400 dark:text-gray-500">Running…</span>
              ) : planEntry.result && (
                <span className="ml-2 font-normal text-gray-400 dark:text-gray-500">
                  {planEntry.result.to_copy} to copy · {planEntry.result.to_skip} to skip · {planEntry.result.files.length} total
                </span>
              )}
            </h2>
            {planEntry.status !== 'running' && (
              <button onClick={() => id && dismissPlan(id)} className="text-xs text-gray-400 hover:text-gray-600 dark:hover:text-gray-300">
                Dismiss
              </button>
            )}
          </div>
          {planEntry.status === 'running' ? (
            <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg px-4 py-8 flex items-center justify-center gap-3 text-sm text-gray-400 dark:text-gray-500">
              <span className="w-4 h-4 border-2 border-current border-t-transparent rounded-full animate-spin" />
              Scanning remote files…
            </div>
          ) : planEntry.result && (
            <PlanTreeView files={planEntry.result.files} remotePath={job?.remote_path ?? ''} />
          )}
        </div>
      )}

      <h2 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-3">Run History</h2>

      {isLoading && <p className="text-gray-500 dark:text-gray-400 text-sm">Loading…</p>}
      {!isLoading && runs.length === 0 && (
        <p className="text-gray-400 dark:text-gray-500 text-sm">No runs yet.</p>
      )}

      <div className="space-y-2">
        {runs.map((run) => (
          <Link
            key={run.id}
            to={`/runs/${run.id}`}
            className="block bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg px-4 py-3 hover:border-blue-300 dark:hover:border-gray-500 transition-colors"
          >
            <div className="flex items-center gap-4">
              <StatusBadge status={run.status} />
              <div className="flex-1 min-w-0">
                <p className="text-xs text-gray-500 dark:text-gray-400">
                  Started {new Date(run.started_at).toLocaleString()}
                  {run.finished_at && ` · Finished ${new Date(run.finished_at).toLocaleString()}`}
                </p>
              </div>
              <div className="flex gap-4 text-xs text-gray-500 dark:text-gray-400">
                <span>{run.total_files} total</span>
                <span className="text-green-600 dark:text-green-400">{run.copied_files} copied</span>
                <span className="text-yellow-600 dark:text-yellow-400">{run.skipped_files} skipped</span>
                {run.failed_files > 0 && <span className="text-red-600 dark:text-red-400">{run.failed_files} failed</span>}
              </div>
            </div>
          </Link>
        ))}
      </div>

      {editOpen && job && (
        <JobFormModal editing={job} onClose={() => setEditOpen(false)} />
      )}
    </div>
  )
}
