import { useState, useEffect, useRef } from 'react'
import { openEventSource } from '../hooks/eventSource'

interface LogEntry {
  time: string
  level: string
  msg: string
  attrs?: Record<string, unknown>
}

function levelClass(level: string): string {
  switch (level.toUpperCase()) {
    case 'ERROR': return 'text-red-500 dark:text-red-400'
    case 'WARN':  return 'text-yellow-500 dark:text-yellow-400'
    case 'DEBUG': return 'text-gray-400 dark:text-gray-500'
    default:      return 'text-gray-700 dark:text-gray-300'
  }
}

function formatAttrs(attrs: Record<string, unknown>): string {
  return Object.entries(attrs)
    .filter(([k]) => k !== 'err' || attrs[k] !== null)
    .map(([k, v]) => `${k}=${JSON.stringify(v)}`)
    .join(' ')
}

export function LogsPage() {
  const [entries, setEntries] = useState<LogEntry[]>([])
  const bottomRef = useRef<HTMLDivElement>(null)
  const [autoScroll, setAutoScroll] = useState(true)

  useEffect(() => {
    return openEventSource('/api/logs/stream', (es) => {
      es.onmessage = (e) => {
        try {
          const entry: LogEntry = JSON.parse(e.data)
          setEntries((prev) => [...prev, entry])
        } catch {
          // ignore malformed
        }
      }
    })
  }, [])

  useEffect(() => {
    if (autoScroll) bottomRef.current?.scrollIntoView({ behavior: 'instant' })
  }, [entries, autoScroll])

  return (
    <div className="flex flex-col h-full">
      <div className="flex items-center justify-between mb-4">
        <h1 className="text-xl font-semibold text-gray-900 dark:text-gray-100">Logs</h1>
        <label className="flex items-center gap-2 text-xs text-gray-500 dark:text-gray-400 cursor-pointer select-none">
          <input
            type="checkbox"
            checked={autoScroll}
            onChange={(e) => setAutoScroll(e.target.checked)}
            className="rounded"
          />
          Auto-scroll
        </label>
      </div>

      <div className="flex-1 card overflow-y-auto font-mono text-xs p-4 space-y-0.5 min-h-0">
        {entries.length === 0 && (
          <p className="text-gray-400 dark:text-gray-500">Waiting for log entries…</p>
        )}
        {entries.map((entry, i) => (
          <div key={i} className="flex gap-2 leading-5">
            <span className="text-gray-400 dark:text-gray-500 shrink-0">
              {new Date(entry.time).toLocaleTimeString()}
            </span>
            <span className={`w-12 shrink-0 font-medium ${levelClass(entry.level)}`}>
              {entry.level.toUpperCase().slice(0, 4)}
            </span>
            <span className="text-gray-800 dark:text-gray-200 break-all">
              {entry.msg}
              {entry.attrs && Object.keys(entry.attrs).length > 0 && (
                <span className="text-gray-500 dark:text-gray-400"> {formatAttrs(entry.attrs)}</span>
              )}
            </span>
          </div>
        ))}
        <div ref={bottomRef} />
      </div>
    </div>
  )
}
