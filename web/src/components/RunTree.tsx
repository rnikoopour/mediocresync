import { useState } from 'react'
import type { Transfer } from '../api/types'
import { StatusBadge } from './StatusBadge'
import { ProgressBar } from './ProgressBar'

export function formatBytes(b: number): string {
  if (b >= 1_073_741_824) return `${(b / 1_073_741_824).toFixed(1)} GB`
  if (b >= 1_048_576)     return `${(b / 1_048_576).toFixed(1)} MB`
  if (b >= 1_024)         return `${(b / 1_024).toFixed(1)} KB`
  return `${b} B`
}

export function formatSpeed(bps: number): string {
  return `${formatBytes(bps)}/s`
}

function sortNodes<T extends { type: string; name: string }>(nodes: T[]): T[] {
  return [...nodes].sort((a, b) => {
    if (a.type !== b.type) return a.type === 'folder' ? -1 : 1
    return a.name.localeCompare(b.name)
  })
}

type RunTreeFile   = { type: 'file'; name: string; transfer: Transfer }
type RunTreeFolder = { type: 'folder'; name: string; children: RunTreeNode[] }
type RunTreeNode   = RunTreeFile | RunTreeFolder

export function buildRunTree(transfers: Transfer[], remotePath: string): RunTreeNode[] {
  const base = remotePath.replace(/\/+$/, '')
  const root: RunTreeFolder = { type: 'folder', name: '', children: [] }

  for (const t of transfers) {
    const rel = t.remote_path.startsWith(base + '/')
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

export type RunTab = 'all' | 'planned' | 'in_progress' | 'copied' | 'not_copied'

type LiveEvent = { percent: number; speed_bps: number; status: string }

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

export function RunTabBar({ tab, onTab, isRunning }: { tab: RunTab; onTab: (t: RunTab) => void; isRunning: boolean }) {
  return (
    <div className="flex flex-wrap items-center gap-1 px-3 py-2 border-b border-gray-100 dark:border-gray-700">
      <TabBtn value="planned"     current={tab} label="Planned"     onTab={onTab} />
      {isRunning && <TabBtn value="in_progress" current={tab} label="In Progress" onTab={onTab} />}
      <TabBtn value="copied"      current={tab} label="Copied"      onTab={onTab} />
      <TabBtn value="not_copied"  current={tab} label="Not Copied"  onTab={onTab} />
      <TabBtn value="all"         current={tab} label="All"         onTab={onTab} />
    </div>
  )
}

function RunFileRow({ node, liveEvents, runEnded }: { node: RunTreeFile; liveEvents: Map<string, LiveEvent>; runEnded: boolean }) {
  const t = node.transfer
  const live = liveEvents.get(t.id)
  const status = live?.status ?? (runEnded && t.status === 'pending' ? 'not_copied' : t.status)
  const speed = live?.speed_bps
  const percent = live?.percent ?? (t.status === 'done' ? 100 : 0)
  const isFailed = status === 'failed'

  return (
    <div className="py-1 hover:bg-gray-50 dark:hover:bg-gray-700/50"
      style={{ paddingLeft: '12px', paddingRight: '16px' }}>
      <div className="flex items-start gap-2">
        <span className="text-gray-400 dark:text-gray-500 shrink-0 mt-0.5">📄</span>
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2">
            <span className="hidden md:inline shrink-0"><StatusBadge status={status} /></span>
            <span className={`font-mono text-xs break-all flex-1 min-w-0 ${isFailed ? 'text-red-500 dark:text-red-400' : 'text-gray-600 dark:text-gray-300'}`}>{node.name}</span>
            <span className="hidden md:inline text-xs text-gray-400 dark:text-gray-500 shrink-0">{formatBytes(t.size_bytes)}</span>
            {status === 'in_progress' && speed !== undefined && speed > 0 && (
              <span className="hidden md:inline text-xs text-gray-400 dark:text-gray-500 shrink-0">{formatSpeed(speed)}</span>
            )}
            {(status === 'in_progress' || status === 'done' || isFailed || status === 'not_copied') && (
              <div className="hidden md:block w-32 shrink-0">
                <ProgressBar
                  percent={isFailed || status === 'not_copied' ? 0 : percent}
                  variant={isFailed ? 'failed' : status === 'not_copied' ? 'not_copied' : 'default'}
                />
              </div>
            )}
          </div>
          <div className="flex md:hidden items-center gap-2 mt-0.5">
            <StatusBadge status={status} />
            <span className="text-xs text-gray-400 dark:text-gray-500">{formatBytes(t.size_bytes)}</span>
            {status === 'in_progress' && speed !== undefined && speed > 0 && (
              <span className="text-xs text-gray-400 dark:text-gray-500">{formatSpeed(speed)}</span>
            )}
          </div>
          {(status === 'in_progress' || status === 'done' || isFailed || status === 'not_copied') && (
            <div className="md:hidden mt-1">
              <ProgressBar
                percent={isFailed || status === 'not_copied' ? 0 : percent}
                variant={isFailed ? 'failed' : status === 'not_copied' ? 'not_copied' : 'default'}
              />
            </div>
          )}
          {isFailed && t.error_msg && (
            <p className="font-mono text-xs text-red-400 dark:text-red-500 break-all mt-0.5">{t.error_msg}</p>
          )}
        </div>
      </div>
    </div>
  )
}

function RunFolderNode({ node, depth, liveEvents, runEnded }: { node: RunTreeFolder; depth: number; liveEvents: Map<string, LiveEvent>; runEnded: boolean }) {
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
              ? <RunFolderNode key={child.name + i} node={child} depth={depth + 1} liveEvents={liveEvents} runEnded={runEnded} />
              : <RunFileRow key={child.name + i} node={child} liveEvents={liveEvents} runEnded={runEnded} />
          )}
        </div>
      )}
    </div>
  )
}

export function RunTreeView({ transfers, remotePath, liveEvents, runEnded, scrollable = true }: {
  transfers: Transfer[]
  remotePath: string
  liveEvents: Map<string, LiveEvent>
  runEnded: boolean
  scrollable?: boolean
}) {
  const [tab, setTab] = useState<RunTab>('planned')
  const filtered = tab === 'all' ? transfers : transfers.filter((t) => {
    const raw = liveEvents.get(t.id)?.status ?? t.status
    const status = runEnded && raw === 'pending' ? 'not_copied' : raw
    if (tab === 'planned')     return status !== 'skipped'
    if (tab === 'in_progress') return status === 'in_progress' || status === 'pending'
    if (tab === 'copied')      return status === 'done'
    if (tab === 'not_copied')  return status === 'not_copied' || status === 'failed' || status === 'canceled'
    return true
  })
  const nodes = buildRunTree(filtered, remotePath)
  return (
    <div className="border-t border-gray-100 dark:border-gray-700">
      <RunTabBar tab={tab} onTab={setTab} isRunning={!runEnded} />
      <div className={`py-1 ${scrollable ? 'max-h-64 overflow-y-auto' : ''}`}>
        {nodes.length === 0
          ? <p className="px-4 py-4 text-xs text-center text-gray-400 dark:text-gray-500">No transfers recorded.</p>
          : nodes.map((n, i) =>
              n.type === 'folder'
                ? <RunFolderNode key={n.name + i} node={n} depth={0} liveEvents={liveEvents} runEnded={runEnded} />
                : <RunFileRow key={n.name + i} node={n} liveEvents={liveEvents} runEnded={runEnded} />
            )
        }
      </div>
    </div>
  )
}
