import { useState, useEffect, useLayoutEffect, useRef } from 'react'
import type { PlanFile } from '../api/types'
import { StatusBadge } from './StatusBadge'
import { formatBytes } from './RunTree'

export type TreeFile = { type: 'file'; name: string; remote_path: string; size_bytes: number; mtime: string; action: 'copy' | 'skip' | 'error' }
type TreeFolder = { type: 'folder'; name: string; children: TreeNode[] }
type TreeNode = TreeFile | TreeFolder

// Folders first, then files, each group alpha-sorted by name.
function sortNodes<T extends { type: string; name: string }>(nodes: T[]): T[] {
  return [...nodes].sort((a, b) => {
    if (a.type !== b.type) return a.type === 'folder' ? -1 : 1
    return a.name.localeCompare(b.name)
  })
}

function buildTree(files: PlanFile[], remotePath: string): TreeNode[] {
  const base = remotePath.replace(/\/+$/, '')
  const root: TreeFolder = { type: 'folder', name: '', children: [] }

  for (const file of files) {
    const rel = file.remote_path.startsWith(base + '/')
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
          className="fixed z-50 card shadow-lg py-1 min-w-[140px]"
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
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2">
          <span className="hidden md:inline shrink-0"><StatusBadge status={node.action === 'copy' ? 'pending' : 'skipped'} /></span>
          <span className="font-mono text-xs text-gray-600 dark:text-gray-300 flex-1 min-w-0 break-all">{node.name}</span>
          <span className="hidden md:inline text-xs text-gray-400 dark:text-gray-500 shrink-0">{formatBytes(node.size_bytes)}</span>
          <button
            className="md:hidden px-1 py-0.5 text-gray-400 dark:text-gray-500 text-base leading-none shrink-0"
            onClick={(e) => { e.stopPropagation(); setMenu({ x: e.clientX, y: e.clientY }) }}
            aria-label="Actions"
          >⋮</button>
        </div>
        <div className="flex md:hidden items-center gap-2 mt-0.5">
          <StatusBadge status={node.action === 'copy' ? 'pending' : 'skipped'} />
          <span className="text-xs text-gray-400 dark:text-gray-500">{formatBytes(node.size_bytes)}</span>
        </div>
      </div>
      {menu && (
        <div
          ref={menuRef}
          className="fixed z-50 card shadow-lg py-1 min-w-[140px]"
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

export function PlanTreeView({ files, remotePath, onSkip, onUnskip }: { files: PlanFile[]; remotePath: string; onSkip: (f: TreeFile) => Promise<void>; onUnskip: (remotePath: string) => Promise<void> }) {
  const [tab, setTab] = useState<TreeTab>('copy')
  const filtered = tab === 'all' ? files : files.filter((f) => tab === 'copy' ? f.action === 'copy' : f.action === 'skip')
  const nodes = buildTree(filtered, remotePath)
  return (
    <div className="card overflow-hidden">
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

function GitRepoRow({ file, onSkip, onUnskip }: {
  file: PlanFile
  onSkip: (path: string, commitHash: string) => Promise<void>
  onUnskip: (path: string) => Promise<void>
}) {
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
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2">
          <span className="hidden md:inline shrink-0"><StatusBadge status={file.action === 'error' ? 'failed' : file.action === 'copy' ? 'pending' : 'skipped'} /></span>
          <span className="font-mono text-xs text-gray-600 dark:text-gray-300 flex-1 min-w-0 break-all">{file.remote_path}</span>
          <button
            className="md:hidden px-1 py-0.5 text-gray-400 dark:text-gray-500 text-base leading-none shrink-0"
            onClick={(e) => { e.stopPropagation(); setMenu({ x: e.clientX, y: e.clientY }) }}
            aria-label="Actions"
          >⋮</button>
        </div>
        <div className="flex md:hidden items-center gap-2 mt-0.5">
          <StatusBadge status={file.action === 'error' ? 'failed' : file.action === 'copy' ? 'pending' : 'skipped'} />
        </div>
        {file.action === 'error' && file.error && (
          <div className="text-xs text-red-500 dark:text-red-400 mt-0.5 break-all">{file.error}</div>
        )}
        {file.action === 'copy' && (file.previous_commit_hash || file.commit_hash) && (
          <div className="font-mono text-xs text-gray-400 dark:text-gray-500 mt-0.5 break-all">
            {file.previous_commit_hash
              ? <>{file.previous_commit_hash.slice(0, 7)} → {file.commit_hash?.slice(0, 7)}</>
              : <>new → {file.commit_hash?.slice(0, 7)}</>
            }
          </div>
        )}
        {file.action === 'skip' && file.commit_hash && (
          <div className="font-mono text-xs text-gray-400 dark:text-gray-500 mt-0.5">
            {file.previous_commit_hash
              ? <>{file.previous_commit_hash.slice(0, 7)} → {file.commit_hash.slice(0, 7)}</>
              : <>new → {file.commit_hash.slice(0, 7)}</>
            }
          </div>
        )}
      </div>
      {menu && (
        <div
          ref={menuRef}
          className="fixed z-50 card shadow-lg py-1 min-w-[140px]"
          style={{ top: menu.y, left: menu.x }}
        >
          {file.action === 'copy' && (
            <button
              className="w-full text-left px-3 py-1.5 text-sm text-gray-700 dark:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-700"
              onClick={() => { onSkip(file.remote_path, file.commit_hash ?? ''); setMenu(null) }}
            >Skip</button>
          )}
          {file.action === 'skip' && (
            <button
              className="w-full text-left px-3 py-1.5 text-sm text-gray-700 dark:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-700"
              onClick={() => { onUnskip(file.remote_path); setMenu(null) }}
            >Unskip</button>
          )}
        </div>
      )}
    </div>
  )
}

export function GitPlanView({ files, onSkip, onUnskip }: {
  files: PlanFile[]
  onSkip: (path: string, commitHash: string) => Promise<void>
  onUnskip: (path: string) => Promise<void>
}) {
  const [tab, setTab] = useState<TreeTab>('copy')
  const filtered = tab === 'all' ? files : files.filter((f) => tab === 'copy' ? f.action === 'copy' : f.action === 'skip')
  return (
    <div className="card overflow-hidden">
      <TreeTabBar tab={tab} onTab={setTab} />
      <div className="divide-y divide-gray-100 dark:divide-gray-700 py-1">
        {filtered.length === 0
          ? <p className="px-4 py-4 text-xs text-center text-gray-400 dark:text-gray-500">No items</p>
          : filtered.map((f) => (
            <GitRepoRow key={f.remote_path} file={f} onSkip={onSkip} onUnskip={onUnskip} />
          ))
        }
      </div>
    </div>
  )
}
