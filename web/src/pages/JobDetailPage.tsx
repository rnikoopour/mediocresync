import { useState, useEffect, useRef } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Link, useParams } from 'react-router-dom'
import { api } from '../api/client'
import type { PlanFile, Run, Transfer } from '../api/types'
import { StatusBadge } from '../components/StatusBadge'
import { ProgressBar } from '../components/ProgressBar'
import { JobFormModal } from '../components/JobFormModal'
import { usePlan } from '../context/PlanContext'
import { useSSE } from '../hooks/useSSE'

function formatBytes(b: number): string {
  if (b >= 1_073_741_824) return `${(b / 1_073_741_824).toFixed(1)} GB`
  if (b >= 1_048_576)     return `${(b / 1_048_576).toFixed(1)} MB`
  if (b >= 1_024)         return `${(b / 1_024).toFixed(1)} KB`
  return `${b} B`
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

function formatSpeed(bps: number): string {
  return `${formatBytes(bps)}/s`
}

function useElapsed(startedAt: string, isRunning: boolean): string {
  const [now, setNow] = useState(() => Date.now())
  useEffect(() => {
    if (!isRunning) return
    const id = setInterval(() => setNow(Date.now()), 1000)
    return () => clearInterval(id)
  }, [isRunning])
  return formatDuration(now - new Date(startedAt).getTime())
}

// Folders first, then files, each group alpha-sorted by name.
function sortNodes<T extends { type: string; name: string }>(nodes: T[]): T[] {
  return [...nodes].sort((a, b) => {
    if (a.type !== b.type) return a.type === 'folder' ? -1 : 1
    return a.name.localeCompare(b.name)
  })
}

// ── Run row (expandable, tree view) ──────────────────────────────────────────

type RunTreeFile = { type: 'file'; name: string; transfer: Transfer }
type RunTreeFolder = { type: 'folder'; name: string; children: RunTreeNode[] }
type RunTreeNode = RunTreeFile | RunTreeFolder

function buildRunTree(transfers: Transfer[], remotePath: string): RunTreeNode[] {
  const base = remotePath.replace(/\/+$/, '')
  const root: RunTreeFolder = { type: 'folder', name: '', children: [] }

  for (const t of transfers) {
    let rel = t.remote_path.startsWith(base + '/')
      ? t.remote_path.slice(base.length + 1)
      : t.remote_path
    const segments = rel.split('/').filter(Boolean)
    if (segments.length === 0) continue

    let cur = root
    for (let i = 0; i < segments.length - 1; i++) {
      const seg = segments[i]
      let child = cur.children.find((c): c is RunTreeFolder => c.type === 'folder' && c.name === seg)
      if (!child) {
        child = { type: 'folder', name: seg, children: [] }
        cur.children.push(child)
      }
      cur = child
    }
    cur.children.push({ type: 'file', name: segments[segments.length - 1], transfer: t })
  }

  function sortFolder(folder: RunTreeFolder) {
    folder.children = sortNodes(folder.children)
    folder.children.forEach((c) => { if (c.type === 'folder') sortFolder(c) })
  }
  sortFolder(root)

  return root.children
}

function RunFileRow({ node, liveEvents }: { node: RunTreeFile; liveEvents: Map<string, { percent: number; speed_bps: number; status: string }> }) {
  const t = node.transfer
  const live = liveEvents.get(t.id)
  const status = live?.status ?? t.status
  const percent = live?.percent ?? (t.status === 'done' ? 100 : 0)
  const speed = live?.speed_bps

  const isFailed = status === 'failed'

  return (
    <div className="py-1 hover:bg-gray-50 dark:hover:bg-gray-700/50"
      style={{ paddingLeft: '12px', paddingRight: '16px' }}>
      <div className="flex flex-wrap items-center gap-2">
        <span className="text-gray-400 dark:text-gray-500 shrink-0">📄</span>
        {(isFailed || status === 'done') && <span className="shrink-0"><StatusBadge status={status} /></span>}
        <span className={`font-mono text-xs flex-1 min-w-0 break-all ${isFailed ? 'text-red-500 dark:text-red-400' : 'text-gray-600 dark:text-gray-300'}`}>{node.name}</span>
        <span className="text-xs text-gray-400 dark:text-gray-500 shrink-0">{formatBytes(t.size_bytes)}</span>
        {status === 'in_progress' && speed !== undefined && speed > 0 && (
          <span className="text-xs text-gray-400 dark:text-gray-500 shrink-0">{formatSpeed(speed)}</span>
        )}
        {(status === 'in_progress' || status === 'done' || isFailed) ? (
          <div className="w-full sm:w-48 shrink-0">
            <ProgressBar percent={isFailed ? 0 : percent} label={isFailed ? 'Failed' : undefined} variant={isFailed ? 'failed' : 'default'} />
          </div>
        ) : (
          <span className="shrink-0"><StatusBadge status={status} /></span>
        )}
      </div>
      {isFailed && t.error_msg && (
        <p className="font-mono text-xs text-red-400 dark:text-red-500 break-all mt-0.5" style={{ paddingLeft: '28px' }}>
          {t.error_msg}
        </p>
      )}
    </div>
  )
}

function RunFolderNode({ node, depth, liveEvents }: { node: RunTreeFolder; depth: number; liveEvents: Map<string, { percent: number; speed_bps: number; status: string }> }) {
  const [open, setOpen] = useState(true)
  const indent = depth * 16

  return (
    <div>
      <button
        onClick={() => setOpen(!open)}
        className="w-full flex items-center gap-2 py-1.5 hover:bg-gray-100 dark:hover:bg-gray-700/60 text-left"
        style={{ paddingLeft: `${16 + indent}px`, paddingRight: '16px' }}
      >
        <span className="text-gray-400 dark:text-violet-500 text-xs w-3 shrink-0">{open ? '▾' : '▸'}</span>
        <span className="shrink-0">📁</span>
        <span className="font-mono text-xs font-semibold text-gray-700 dark:text-violet-300">{node.name}</span>
      </button>
      {open && (
        <div className="border-l border-blue-100 dark:border-gray-600" style={{ marginLeft: `${16 + indent + 12}px` }}>
          {node.children.map((child, i) =>
            child.type === 'folder'
              ? <RunFolderNode key={child.name + i} node={child} depth={depth + 1} liveEvents={liveEvents} />
              : <RunFileRow key={child.name + i} node={child} liveEvents={liveEvents} />
          )}
        </div>
      )}
    </div>
  )
}

function RunTreeView({ transfers, remotePath, liveEvents }: {
  transfers: Transfer[]
  remotePath: string
  liveEvents: Map<string, { percent: number; speed_bps: number; status: string }>
}) {
  const [tab, setTab] = useState<TreeTab>('copy')
  const filtered = tab === 'all' ? transfers : transfers.filter((t) => {
    const status = liveEvents.get(t.id)?.status ?? t.status
    return tab === 'skip' ? status === 'skipped' : status !== 'skipped'
  })
  const nodes = buildRunTree(filtered, remotePath)
  return (
    <div className="border-t border-gray-100 dark:border-gray-700">
      <TreeTabBar tab={tab} onTab={setTab} />
      <div className="py-1 max-h-64 overflow-y-auto">
        {nodes.map((n, i) =>
          n.type === 'folder'
            ? <RunFolderNode key={n.name + i} node={n} depth={0} liveEvents={liveEvents} />
            : <RunFileRow key={n.name + i} node={n} liveEvents={liveEvents} />
        )}
      </div>
    </div>
  )
}

function RunRow({ run: initialRun, remotePath, jobId }: { run: Run; remotePath: string; jobId: string }) {
  const qc = useQueryClient()
  const [open, setOpen] = useState(initialRun.status === 'running')

  const { data: run = initialRun } = useQuery({
    queryKey: ['run', initialRun.id],
    queryFn: () => api.runs.get(initialRun.id),
    enabled: open,
    refetchInterval: (q) => q.state.data?.status === 'running' ? 3000 : false,
  })

  const [cancelling, setCancelling] = useState(false)
  const cancel = useMutation({
    mutationFn: () => api.jobs.cancel(jobId),
    onMutate: () => setCancelling(true),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['run', initialRun.id] })
      qc.invalidateQueries({ queryKey: ['runs', jobId] })
    },
    onError: () => setCancelling(false),
  })

  const { events: liveEvents, runStatus } = useSSE(open && run.status === 'running' ? run.id : null)
  const effectiveStatus = runStatus ?? run.status
  const isRunning = effectiveStatus === 'running'
  const elapsed = useElapsed(run.started_at, isRunning)

  // Reset cancelling flag once we know the run is no longer running.
  if (cancelling && !isRunning) setCancelling(false)

  const transfers = run.transfers // undefined until detail fetch completes

  const duration = run.finished_at
    ? formatDuration(new Date(run.finished_at).getTime() - new Date(run.started_at).getTime())
    : isRunning ? elapsed : null

  // Live total speed: sum speed_bps of all in-progress transfers.
  const liveSpeedBps = isRunning
    ? Array.from(liveEvents.values()).reduce((s, e) => e.status === 'in_progress' ? s + e.speed_bps : s, 0)
    : 0

  // Average speed for finished runs: total bytes ÷ duration.
  const avgSpeedBps = !isRunning && run.finished_at && run.total_size_bytes > 0
    ? run.total_size_bytes / ((new Date(run.finished_at).getTime() - new Date(run.started_at).getTime()) / 1000)
    : null

  return (
    <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg overflow-hidden">
      <div className="flex flex-wrap items-center gap-4 px-4 py-3">
        <button
          onClick={() => setOpen((o) => !o)}
          className="flex flex-wrap items-center gap-4 flex-1 min-w-0 hover:opacity-80 transition-opacity text-left"
        >
          <StatusBadge status={effectiveStatus} />
          <div className="flex-1 min-w-0">
            <p className="text-xs text-gray-500 dark:text-gray-400">
              Started {new Date(run.started_at).toLocaleString()}
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
          <span className="text-gray-400 dark:text-gray-500 text-xs w-3 shrink-0">{open ? '▾' : '▸'}</span>
        </button>
        {(effectiveStatus === 'running' || cancelling) && (
          <button
            onClick={() => cancel.mutate()}
            disabled={cancelling}
            className="btn-danger text-xs shrink-0"
          >
            {cancelling ? 'Cancelling…' : 'Cancel'}
          </button>
        )}
      </div>

      {open && (
        transfers === undefined ? (
          <p className="border-t border-gray-100 dark:border-gray-700 px-4 py-4 text-xs text-center text-gray-400 dark:text-gray-500">
            Processing plan…
          </p>
        ) : transfers.length === 0 ? (
          <p className="border-t border-gray-100 dark:border-gray-700 px-4 py-4 text-xs text-center text-gray-400 dark:text-gray-500">
            No transfers recorded.
          </p>
        ) : (
          <RunTreeView transfers={transfers} remotePath={remotePath} liveEvents={liveEvents} />
        )
      )}
    </div>
  )
}

// ── Plan tree ────────────────────────────────────────────────────────────────

type TreeFile = { type: 'file'; name: string; remote_path: string; size_bytes: number; action: 'copy' | 'skip' }
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
    cur.children.push({ type: 'file', name: segments[segments.length - 1], remote_path: file.remote_path, size_bytes: file.size_bytes, action: file.action })
  }

  function sortFolder(folder: TreeFolder) {
    folder.children = sortNodes(folder.children)
    folder.children.forEach((c) => { if (c.type === 'folder') sortFolder(c) })
  }
  sortFolder(root)

  return root.children
}

function FolderNode({ node, depth, onUnskip }: { node: TreeFolder; depth: number; onUnskip: (remotePath: string) => void }) {
  const [open, setOpen] = useState(true)
  const indent = depth * 16

  return (
    <div>
      <button
        onClick={() => setOpen(!open)}
        className="w-full flex items-center gap-2 py-1.5 hover:bg-gray-100 dark:hover:bg-gray-700/60 text-left"
        style={{ paddingLeft: `${16 + indent}px`, paddingRight: '16px' }}
      >
        <span className="text-gray-400 dark:text-violet-500 text-xs w-3 shrink-0">{open ? '▾' : '▸'}</span>
        <span className="shrink-0">📁</span>
        <span className="font-mono text-xs font-semibold text-gray-700 dark:text-violet-300">{node.name}</span>
      </button>
      {open && (
        <div className="border-l border-blue-100 dark:border-gray-600" style={{ marginLeft: `${16 + indent + 12}px` }}>
          {node.children.map((child, i) =>
            child.type === 'folder'
              ? <FolderNode key={child.name + i} node={child} depth={depth + 1} onUnskip={onUnskip} />
              : <FileRow key={child.name + i} node={child} depth={depth + 1} onUnskip={onUnskip} />
          )}
        </div>
      )}
    </div>
  )
}

function FileRow({ node, onUnskip }: { node: TreeFile; depth: number; onUnskip?: (remotePath: string) => void }) {
  const [menu, setMenu] = useState<{ x: number; y: number } | null>(null)
  const menuRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!menu) return
    function close(e: MouseEvent) {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) setMenu(null)
    }
    document.addEventListener('mousedown', close)
    return () => document.removeEventListener('mousedown', close)
  }, [menu])

  return (
    <div
      className="flex items-center gap-4 py-1 hover:bg-gray-50 dark:hover:bg-gray-700/50 relative"
      style={{ paddingLeft: '12px', paddingRight: '16px' }}
      onContextMenu={node.action === 'skip' ? (e) => { e.preventDefault(); setMenu({ x: e.clientX, y: e.clientY }) } : undefined}
    >
      <span className="text-gray-400 dark:text-gray-500 shrink-0">📄</span>
      <span className="font-mono text-xs text-gray-600 dark:text-gray-300 flex-1 min-w-0 break-all">{node.name}</span>
      <span className="text-xs text-gray-400 dark:text-gray-500 shrink-0">{formatBytes(node.size_bytes)}</span>
      <span className="shrink-0"><StatusBadge status={node.action === 'copy' ? 'pending' : 'skipped'} /></span>
      {menu && (
        <div
          ref={menuRef}
          className="fixed z-50 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg shadow-lg py-1 min-w-[140px]"
          style={{ top: menu.y, left: menu.x }}
        >
          <button
            className="w-full text-left px-3 py-1.5 text-sm text-gray-700 dark:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-700"
            onClick={() => { onUnskip?.(node.remote_path); setMenu(null) }}
          >
            Force copy
          </button>
        </div>
      )}
    </div>
  )
}

type TreeTab = 'all' | 'copy' | 'skip'

function TreeTabBar({ tab, onTab }: { tab: TreeTab; onTab: (t: TreeTab) => void }) {
  const btn = (t: TreeTab, label: string) => (
    <button
      onClick={() => onTab(t)}
      className={`px-3 py-1.5 text-xs font-medium rounded transition-colors ${
        tab === t
          ? 'bg-gray-200 dark:bg-gray-600 text-gray-900 dark:text-gray-100'
          : 'text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300'
      }`}
    >
      {label}
    </button>
  )
  return (
    <div className="flex items-center gap-1 px-3 py-2 border-b border-gray-100 dark:border-gray-700">
      {btn('copy', 'To Copy')}
      {btn('skip', 'Skipped')}
      {btn('all', 'All')}
    </div>
  )
}

function PlanTreeView({ files, remotePath, onUnskip }: { files: PlanFile[]; remotePath: string; onUnskip: (remotePath: string) => void }) {
  const [tab, setTab] = useState<TreeTab>('copy')
  const filtered = tab === 'all' ? files : files.filter((f) => tab === 'copy' ? f.action === 'copy' : f.action === 'skip')
  const nodes = buildTree(filtered, remotePath)
  return (
    <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg overflow-hidden">
      <TreeTabBar tab={tab} onTab={setTab} />
      <div className="py-1">
        {nodes.map((n, i) =>
          n.type === 'folder'
            ? <FolderNode key={n.name + i} node={n} depth={0} onUnskip={onUnskip} />
            : <FileRow key={n.name + i} node={n} depth={0} onUnskip={onUnskip} />
        )}
      </div>
    </div>
  )
}

export function JobDetailPage() {
  const { id } = useParams<{ id: string }>()
  const qc = useQueryClient()

  const { data: job } = useQuery({ queryKey: ['jobs', id], queryFn: () => api.jobs.get(id!) })
  const { data: runs = [], isLoading } = useQuery({
    queryKey: ['runs', id],
    queryFn: () => api.jobs.listRuns(id!),
    refetchInterval: (q) => q.state.data?.[0]?.status === 'running' ? 3000 : false,
  })
  const { plans, runPlan, subscribePlan, dismissPlan, unskipFile } = usePlan()
  const planEntry = id ? plans[id] : undefined

  // Auto-subscribe to plan events so plans started by other clients are visible.
  // The cleanup closes the EventSource when the page unmounts or id changes.
  useEffect(() => {
    if (!id) return
    return subscribePlan(id)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id])
  const [editOpen, setEditOpen] = useState(false)
  const jobIsRunning = runs[0]?.status === 'running'

  const trigger = useMutation({
    mutationFn: () => api.jobs.trigger(id!),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['runs', id] })
      if (id) dismissPlan(id)
    },
  })

  const unskip = useMutation({
    mutationFn: (remotePath: string) => api.jobs.deleteFileState(id!, remotePath),
    onSuccess: (_, remotePath) => id && unskipFile(id, remotePath),
  })

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
                {job.remote_path} → {job.local_dest}
              </p>
              <p className="text-xs text-gray-500 dark:text-gray-400">
                Every {job.interval_value} {job.interval_unit} · {job.concurrency} concurrent · autosync {job.enabled ? 'enabled' : 'disabled'}
              </p>
            </>
          )}
        </div>
        <div className="flex flex-wrap gap-2">
          <button onClick={() => setEditOpen(true)} className="btn-secondary">Edit</button>
          <button
            onClick={() => id && runPlan(id)}
            disabled={planEntry?.status === 'running' || jobIsRunning}
            className="btn-secondary"
          >
            {planEntry?.status === 'running' ? 'Planning…' : 'Plan'}
          </button>
          <button
            onClick={() => trigger.mutate()}
            disabled={trigger.isPending || planEntry?.status !== 'done' || jobIsRunning}
            title={jobIsRunning ? 'A run is already in progress' : planEntry?.status !== 'done' ? 'Plan first before running' : undefined}
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
                  {' · '}{formatBytes(planEntry.result.files.filter(f => f.action === 'copy').reduce((s, f) => s + f.size_bytes, 0))}
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
              <span className="w-4 h-4 border-2 border-current border-t-transparent rounded-full animate-spin shrink-0" />
              Scanning…
              {(planEntry.scannedFiles > 0 || planEntry.scannedFolders > 0) && (
                <span>{planEntry.scannedFiles} files, {planEntry.scannedFolders} folders found</span>
              )}
            </div>
          ) : planEntry.result && (
            <PlanTreeView files={planEntry.result.files} remotePath={job?.remote_path ?? ''} onUnskip={(p) => unskip.mutate(p)} />
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
          <RunRow key={run.id} run={run} remotePath={job?.remote_path ?? ''} jobId={id!} />
        ))}
      </div>

      {editOpen && job && (
        <JobFormModal editing={job} onClose={() => setEditOpen(false)} />
      )}
    </div>
  )
}
