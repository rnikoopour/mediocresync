import { useState, useEffect, useLayoutEffect, useRef } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Link, useParams } from 'react-router-dom'
import { api } from '../api/client'
import type { PlanFile, Run } from '../api/types'
import { StatusBadge } from '../components/StatusBadge'
import { JobFormModal } from '../components/JobFormModal'
import { RunTreeView, formatBytes, formatSpeed } from '../components/RunTree'

import { usePlan } from '../context/PlanContext'
import { useSSE } from '../hooks/useSSE'
import { openEventSource } from '../hooks/eventSource'

function formatDuration(ms: number): string {
  const s = Math.floor(ms / 1000)
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  const sec = s % 60
  if (h > 0) return `${h}h ${m}m ${sec}s`
  if (m > 0) return `${m}m ${sec}s`
  return `${sec}s`
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

// ── Run row ───────────────────────────────────────────────────────────────────

function RunRow({ run: initialRun, remotePath, jobId }: { run: Run; remotePath: string; jobId: string }) {
  const qc = useQueryClient()
  const [open, setOpen] = useState(initialRun.status === 'running')

  const { data: run = initialRun } = useQuery({
    queryKey: ['run', initialRun.id],
    queryFn: () => api.runs.get(initialRun.id),
    enabled: open,
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

  // When the run finishes (SSE run_status fires), fetch the final state.
  useEffect(() => {
    if (runStatus) qc.invalidateQueries({ queryKey: ['run', initialRun.id] })
  }, [runStatus, initialRun.id, qc])
  const effectiveStatus = (runStatus && runStatus !== 'canceling') ? runStatus : run.status
  const isRunning = effectiveStatus === 'running'
  // Cancelling if this client requested it OR if the server broadcast that
  // another client requested cancellation.
  const isCancelling = cancelling || runStatus === 'canceling'
  const elapsed = useElapsed(run.started_at, isRunning)

  // Reset local cancelling flag once we know the run is no longer running.
  if (cancelling && !isRunning && runStatus !== 'canceling') setCancelling(false)

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
        <Link
          to={`/runs/${run.id}`}
          onClick={(e) => e.stopPropagation()}
          className="text-gray-400 dark:text-gray-500 hover:text-gray-600 dark:hover:text-gray-300 text-xs shrink-0"
          title="View run details"
        >
          ↗
        </Link>
        {(effectiveStatus === 'running' || isCancelling) && (
          <button
            onClick={() => cancel.mutate()}
            disabled={isCancelling}
            className="btn-danger text-xs shrink-0"
          >
            {isCancelling ? 'Cancelling…' : 'Cancel'}
          </button>
        )}
      </div>

      {open && (
        transfers === undefined ? (
          <p className="border-t border-gray-100 dark:border-gray-700 px-4 py-4 text-xs text-center text-gray-400 dark:text-gray-500">
            Processing plan…
          </p>
        ) : transfers.length === 0 ? (
          <div className="border-t border-gray-100 dark:border-gray-700 px-4 py-4 text-xs text-center text-gray-400 dark:text-gray-500">
            {run.error_msg
              ? <p className="text-red-500 dark:text-red-400 font-mono break-all">{run.error_msg}</p>
              : <p>No transfers recorded.</p>
            }
          </div>
        ) : (
          <RunTreeView transfers={transfers} remotePath={remotePath} liveEvents={liveEvents} runEnded={!isRunning} />
        )
      )}
    </div>
  )
}

// ── Plan tree ────────────────────────────────────────────────────────────────

type TreeFile = { type: 'file'; name: string; remote_path: string; size_bytes: number; mtime: string; action: 'copy' | 'skip' }
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
    cur.children.push({ type: 'file', name: segments[segments.length - 1], remote_path: file.remote_path, size_bytes: file.size_bytes, mtime: file.mtime, action: file.action })
  }

  function sortFolder(folder: TreeFolder) {
    folder.children = sortNodes(folder.children)
    folder.children.forEach((c) => { if (c.type === 'folder') sortFolder(c) })
  }
  sortFolder(root)

  return root.children
}

function collectFiles(folder: TreeFolder): TreeFile[] {
  const result: TreeFile[] = []
  for (const child of folder.children) {
    if (child.type === 'file') result.push(child)
    else result.push(...collectFiles(child))
  }
  return result
}

function FolderNode({ node, depth, onSkip, onUnskip }: { node: TreeFolder; depth: number; onSkip: (f: TreeFile) => Promise<void>; onUnskip: (remotePath: string) => Promise<void> }) {
  const [open, setOpen] = useState(true)
  const [menu, setMenu] = useState<{ x: number; y: number } | null>(null)
  const menuRef = useRef<HTMLDivElement>(null)
  const indent = depth * 16

  useEffect(() => {
    if (!menu) return
    function close(e: MouseEvent) {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) setMenu(null)
    }
    document.addEventListener('mousedown', close)
    return () => document.removeEventListener('mousedown', close)
  }, [menu])

  useLayoutEffect(() => {
    if (!menu || !menuRef.current) return
    const el = menuRef.current
    const r = el.getBoundingClientRect()
    if (r.right  > window.innerWidth)  el.style.left = `${menu.x - r.width}px`
    if (r.bottom > window.innerHeight) el.style.top  = `${menu.y - r.height}px`
  }, [menu])

  const leaves = collectFiles(node)
  const hasCopy = leaves.some((f) => f.action === 'copy')
  const hasSkip = leaves.some((f) => f.action === 'skip')

  return (
    <div>
      <div className="flex items-center hover:bg-gray-100 dark:hover:bg-gray-700/60" style={{ paddingRight: '4px' }}>
        <button
          onClick={() => setOpen(!open)}
          onContextMenu={(e) => { e.preventDefault(); e.stopPropagation(); setMenu({ x: e.clientX, y: e.clientY }) }}
          className="flex-1 flex items-center gap-2 py-1.5 text-left"
          style={{ paddingLeft: `${16 + indent}px` }}
        >
          <span className="text-gray-400 dark:text-violet-500 text-xs w-3 shrink-0">{open ? '▾' : '▸'}</span>
          <span className="shrink-0">📁</span>
          <span className="font-mono text-xs font-semibold text-gray-700 dark:text-violet-300">{node.name}</span>
        </button>
        {(hasCopy || hasSkip) && (
          <button
            className="md:hidden px-2 py-1 text-gray-400 dark:text-gray-500 text-base leading-none"
            onClick={(e) => setMenu({ x: e.clientX, y: e.clientY })}
            aria-label="Actions"
          >⋮</button>
        )}
      </div>
      {menu && (
        <div
          ref={menuRef}
          className="fixed z-50 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg shadow-lg py-1 min-w-[140px]"
          style={{ top: menu.y, left: menu.x }}
        >
          {hasCopy && (
            <button
              className="w-full text-left px-3 py-1.5 text-sm text-gray-700 dark:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-700"
              onClick={() => { setMenu(null); leaves.filter((f) => f.action === 'copy').reduce((chain, f) => chain.then(() => onSkip(f)), Promise.resolve()) }}
            >
              Skip all
            </button>
          )}
          {hasSkip && (
            <button
              className="w-full text-left px-3 py-1.5 text-sm text-gray-700 dark:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-700"
              onClick={() => { setMenu(null); leaves.filter((f) => f.action === 'skip').reduce((chain, f) => chain.then(() => onUnskip(f.remote_path)), Promise.resolve()) }}
            >
              Unskip all
            </button>
          )}
        </div>
      )}
      {open && (
        <div className="border-l border-blue-100 dark:border-gray-600" style={{ marginLeft: `${16 + indent + 12}px` }}>
          {node.children.map((child, i) =>
            child.type === 'folder'
              ? <FolderNode key={child.name + i} node={child} depth={depth + 1} onSkip={onSkip} onUnskip={onUnskip} />
              : <FileRow key={child.name + i} node={child} depth={depth + 1} onSkip={onSkip} onUnskip={onUnskip} />
          )}
        </div>
      )}
    </div>
  )
}

function FileRow({ node, onSkip, onUnskip }: { node: TreeFile; depth: number; onSkip?: (f: TreeFile) => Promise<void>; onUnskip?: (remotePath: string) => Promise<void> }) {
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

  useLayoutEffect(() => {
    if (!menu || !menuRef.current) return
    const el = menuRef.current
    const r = el.getBoundingClientRect()
    if (r.right  > window.innerWidth)  el.style.left = `${menu.x - r.width}px`
    if (r.bottom > window.innerHeight) el.style.top  = `${menu.y - r.height}px`
  }, [menu])

  return (
    <div
      className="flex items-center gap-2 py-1 hover:bg-gray-50 dark:hover:bg-gray-700/50 relative"
      style={{ paddingLeft: '12px', paddingRight: '16px' }}
      onContextMenu={(e) => { e.preventDefault(); setMenu({ x: e.clientX, y: e.clientY }) }}
    >
      <span className="text-gray-400 dark:text-gray-500 shrink-0">📄</span>
      {/* inner column: name + (mobile) second line with size & badge */}
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2">
          {/* desktop: badge left of filename */}
          <span className="hidden md:inline shrink-0"><StatusBadge status={node.action === 'copy' ? 'pending' : 'skipped'} /></span>
          <span className="font-mono text-xs text-gray-600 dark:text-gray-300 flex-1 min-w-0 break-all">{node.name}</span>
          {/* desktop: size right of filename */}
          <span className="hidden md:inline text-xs text-gray-400 dark:text-gray-500 shrink-0">{formatBytes(node.size_bytes)}</span>
          <button
            className="md:hidden px-1 py-0.5 text-gray-400 dark:text-gray-500 text-base leading-none shrink-0"
            onClick={(e) => { e.stopPropagation(); setMenu({ x: e.clientX, y: e.clientY }) }}
            aria-label="Actions"
          >⋮</button>
        </div>
        {/* mobile: badge + size on second line */}
        <div className="flex md:hidden items-center gap-2 mt-0.5">
          <StatusBadge status={node.action === 'copy' ? 'pending' : 'skipped'} />
          <span className="text-xs text-gray-400 dark:text-gray-500">{formatBytes(node.size_bytes)}</span>
        </div>
      </div>
      {menu && (
        <div
          ref={menuRef}
          className="fixed z-50 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg shadow-lg py-1 min-w-[140px]"
          style={{ top: menu.y, left: menu.x }}
        >
          {node.action === 'copy' && (
            <button
              className="w-full text-left px-3 py-1.5 text-sm text-gray-700 dark:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-700"
              onClick={() => { onSkip?.(node); setMenu(null) }}
            >
              Skip
            </button>
          )}
          {node.action === 'skip' && (
            <button
              className="w-full text-left px-3 py-1.5 text-sm text-gray-700 dark:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-700"
              onClick={() => { onUnskip?.(node.remote_path); setMenu(null) }}
            >
              Unskip
            </button>
          )}
        </div>
      )}
    </div>
  )
}

type TreeTab = 'all' | 'copy' | 'skip'

function TabBtn<T extends string>({ value, current, label, onTab }: { value: T; current: T; label: string; onTab: (t: T) => void }) {
  return (
    <button
      onClick={() => onTab(value)}
      className={`px-3 py-1.5 text-xs font-medium rounded transition-colors ${
        current === value
          ? 'bg-gray-200 dark:bg-gray-600 text-gray-900 dark:text-gray-100'
          : 'text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300'
      }`}
    >
      {label}
    </button>
  )
}

function TreeTabBar({ tab, onTab }: { tab: TreeTab; onTab: (t: TreeTab) => void }) {
  return (
    <div className="flex items-center gap-1 px-3 py-2 border-b border-gray-100 dark:border-gray-700">
      <TabBtn value="copy" current={tab} label="Planned" onTab={onTab} />
      <TabBtn value="skip" current={tab} label="Skipped" onTab={onTab} />
      <TabBtn value="all"  current={tab} label="All"     onTab={onTab} />
    </div>
  )
}

function PlanTreeView({ files, remotePath, onSkip, onUnskip }: { files: PlanFile[]; remotePath: string; onSkip: (f: TreeFile) => Promise<void>; onUnskip: (remotePath: string) => Promise<void> }) {
  const [tab, setTab] = useState<TreeTab>('copy')
  const filtered = tab === 'all' ? files : files.filter((f) => tab === 'copy' ? f.action === 'copy' : f.action === 'skip')
  const nodes = buildTree(filtered, remotePath)
  return (
    <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg overflow-hidden">
      <TreeTabBar tab={tab} onTab={setTab} />
      <div className="py-1">
        {nodes.length === 0
          ? <p className="px-4 py-4 text-xs text-center text-gray-400 dark:text-gray-500">No items to copy</p>
          : nodes.map((n, i) =>
              n.type === 'folder'
                ? <FolderNode key={n.name + i} node={n} depth={0} onSkip={onSkip} onUnskip={onUnskip} />
                : <FileRow key={n.name + i} node={n} depth={0} onSkip={onSkip} onUnskip={onUnskip} />
            )
        }
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
  })
  const { plans, runPlan, subscribePlan, dismissPlan, unskipFile, skipFile } = usePlan()
  const planEntry = id ? plans[id] : undefined
  const [planOpen, setPlanOpen] = useState(true)

  // Auto-subscribe to plan events so plans started by other clients are visible.
  // The cleanup closes the EventSource when the page unmounts or id changes.
  useEffect(() => {
    if (!id) return
    return subscribePlan(id)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id])

  // Subscribe to job-level events so runs runed by other clients (including
  // the scheduler) are discovered immediately. On reconnect after a visibility
  // change or error, refetch runs to catch any events missed while disconnected.
  useEffect(() => {
    if (!id) return
    return openEventSource(`/api/jobs/${id}/events`, (es) => {
      es.onopen = () => {
        qc.invalidateQueries({ queryKey: ['runs', id] })
      }
      es.onmessage = (e) => {
        const ev = JSON.parse(e.data)
        if (ev.status === 'started' || ev.status === 'run_finished' || ev.status === 'runs_pruned') {
          qc.invalidateQueries({ queryKey: ['runs', id] })
        } else if (ev.status === 'plan_file_updated') {
          if (ev.plan_action === 'skip') skipFile(id, ev.plan_path)
          else unskipFile(id, ev.plan_path)
        }
      }
    })
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id])
  const [editOpen, setEditOpen] = useState(false)
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
                {job.remote_path} → {job.local_dest}
              </p>
              <p className="text-xs text-gray-500 dark:text-gray-400">
                Every {job.interval_value} {job.interval_unit} · {job.concurrency} concurrent · autosync {job.enabled ? 'enabled' : 'disabled'}
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
        <div className="mb-8 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg overflow-hidden">
          <div className="flex items-center gap-4 px-4 py-3">
            <button
              onClick={() => setPlanOpen((o) => !o)}
              className="flex flex-wrap items-center gap-4 flex-1 min-w-0 hover:opacity-80 transition-opacity text-left"
            >
              <span className="text-sm font-medium text-gray-700 dark:text-gray-300">Plan Result</span>
              {planEntry.status === 'running' ? (
                <span className="text-xs font-normal text-gray-400 dark:text-gray-500">Running…</span>
              ) : planEntry.result && (
                <span className="text-xs font-normal text-gray-400 dark:text-gray-500">
                  {planEntry.result.to_copy} to copy · {planEntry.result.to_skip} to skip · {planEntry.result.files.length} total
                  {' · '}{formatBytes(planEntry.result.files.filter(f => f.action === 'copy').reduce((s, f) => s + f.size_bytes, 0))}
                </span>
              )}
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
                <PlanTreeView files={planEntry.result.files} remotePath={job?.remote_path ?? ''} onSkip={doSkip} onUnskip={doUnskip} />
              </div>
            )
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
