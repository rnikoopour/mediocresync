import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import type { BrowseEntry } from '../api/types'

interface Props {
  title: string
  initialPath: string
  queryKey: unknown[]
  fetcher: (path: string) => Promise<BrowseEntry[]>
  onSelect: (path: string) => void
  onClose: () => void
}

export function FileBrowser({ title, initialPath, queryKey, fetcher, onSelect, onClose }: Props) {
  const [currentPath, setCurrentPath] = useState(initialPath)

  const { data: entries = [], isLoading, isError } = useQuery({
    queryKey: [...queryKey, currentPath],
    queryFn: () => fetcher(currentPath),
  })

  const segments = currentPath.replace(/\\/g, '/').split('/').filter(Boolean)

  function navigate(path: string) {
    setCurrentPath(path || '/')
  }

  function navigateUp() {
    const parts = currentPath.replace(/\\/g, '/').split('/').filter(Boolean)
    const parent = '/' + parts.slice(0, -1).join('/')
    navigate(parent || '/')
  }

  const dirs = entries.filter((e) => e.is_dir)
  const files = entries.filter((e) => !e.is_dir)

  return (
    <div className="fixed inset-0 bg-black/40 flex items-center justify-center z-50">
      <div className="bg-white dark:bg-gray-800 rounded-xl shadow-xl w-full max-w-lg mx-4 flex flex-col" style={{ maxHeight: '80vh' }}>
        {/* Header */}
        <div className="flex items-center justify-between px-4 py-3 border-b border-gray-200 dark:border-gray-700">
          <h2 className="font-semibold text-gray-900 dark:text-gray-100 text-sm">{title}</h2>
          <button onClick={onClose} className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 text-xl leading-none">&times;</button>
        </div>

        {/* Breadcrumb */}
        <div className="flex items-center gap-1 px-4 py-2 border-b border-gray-100 dark:border-gray-700 text-xs overflow-x-auto whitespace-nowrap">
          <button onClick={() => navigate('/')} className="text-blue-600 dark:text-blue-400 hover:underline font-medium">/</button>
          {segments.map((seg, i) => {
            const path = '/' + segments.slice(0, i + 1).join('/')
            const isLast = i === segments.length - 1
            return (
              <span key={path} className="flex items-center gap-1">
                <span className="text-gray-400 dark:text-gray-500">/</span>
                {isLast ? (
                  <span className="text-gray-700 dark:text-gray-200 font-medium">{seg}</span>
                ) : (
                  <button onClick={() => navigate(path)} className="text-blue-600 dark:text-blue-400 hover:underline">{seg}</button>
                )}
              </span>
            )
          })}
        </div>

        {/* Entries */}
        <div className="flex-1 overflow-y-auto">
          {isLoading && <p className="text-gray-400 text-sm px-4 py-6 text-center">Loading…</p>}
          {isError && <p className="text-red-500 text-sm px-4 py-6 text-center">Failed to list directory.</p>}
          {!isLoading && !isError && (
            <ul className="divide-y divide-gray-100 dark:divide-gray-700">
              {segments.length > 0 && (
                <li>
                  <button
                    onClick={navigateUp}
                    className="w-full flex items-center gap-2 px-4 py-2 hover:bg-gray-50 dark:hover:bg-gray-700 text-sm text-gray-500 dark:text-gray-400"
                  >
                    <FolderIcon className="text-gray-400 dark:text-gray-500" />
                    ..
                  </button>
                </li>
              )}
              {dirs.map((e) => (
                <li key={e.path}>
                  <button
                    onClick={() => navigate(e.path)}
                    className="w-full flex items-center gap-2 px-4 py-2 hover:bg-gray-50 dark:hover:bg-gray-700 text-sm text-gray-700 dark:text-gray-200 text-left"
                  >
                    <FolderIcon className="text-blue-400" />
                    {e.name}
                  </button>
                </li>
              ))}
              {files.map((e) => (
                <li key={e.path}>
                  <div className="flex items-center gap-2 px-4 py-2 text-sm text-gray-400 dark:text-gray-500">
                    <FileIcon />
                    {e.name}
                  </div>
                </li>
              ))}
              {dirs.length === 0 && files.length === 0 && (
                <li className="px-4 py-6 text-center text-gray-400 dark:text-gray-500 text-sm">Empty directory</li>
              )}
            </ul>
          )}
        </div>

        {/* Footer */}
        <div className="flex items-center justify-between px-4 py-3 border-t border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-900/50 rounded-b-xl">
          <span className="text-xs text-gray-500 dark:text-gray-400 font-mono truncate mr-4">{currentPath}</span>
          <div className="flex gap-2 shrink-0">
            <button type="button" onClick={onClose} className="btn-secondary text-xs">Cancel</button>
            <button type="button" onClick={() => { onSelect(currentPath); onClose() }} className="btn-primary text-xs">
              Select
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}

function FolderIcon({ className }: { className?: string }) {
  return (
    <svg className={`w-4 h-4 shrink-0 ${className ?? ''}`} fill="currentColor" viewBox="0 0 20 20">
      <path d="M2 6a2 2 0 012-2h4l2 2h6a2 2 0 012 2v6a2 2 0 01-2 2H4a2 2 0 01-2-2V6z" />
    </svg>
  )
}

function FileIcon() {
  return (
    <svg className="w-4 h-4 shrink-0 text-gray-300 dark:text-gray-600" fill="currentColor" viewBox="0 0 20 20">
      <path fillRule="evenodd" d="M4 4a2 2 0 012-2h4.586A2 2 0 0112 2.586L15.414 6A2 2 0 0116 7.414V16a2 2 0 01-2 2H6a2 2 0 01-2-2V4z" clipRule="evenodd" />
    </svg>
  )
}
