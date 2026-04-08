import { createContext, useContext, useRef } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { openEventSource } from '../hooks/eventSource'
import type { ProgressEvent } from '../api/types'
import { isTerminalStatus } from '../utils/runStatus'

export interface RunSnapshot {
  events: Map<string, ProgressEvent>
  runStatus: string | null
  isDone: boolean
}

export const EMPTY_SNAPSHOT: RunSnapshot = Object.freeze({
  events: new Map(),
  runStatus: null,
  isDone: false,
})

interface RunEntry {
  // Snapshot is replaced (new object) on every change so useSyncExternalStore
  // can detect updates via reference equality.
  snapshot: RunSnapshot
  jobId: string
}

interface RunStateContextValue {
  // For useSyncExternalStore — subscribe to a specific run's changes.
  subscribeToRun: (runID: string, callback: () => void) => () => void
  // For useSyncExternalStore — read the current snapshot for a run.
  getSnapshot: (runID: string) => RunSnapshot
  // Open/ref-count an SSE connection for a run. Returns an unsubscribe fn.
  openSSE: (runID: string, jobID: string) => () => void
}

const RunStateContext = createContext<RunStateContextValue | null>(null)

export function RunStateProvider({ children }: { children: React.ReactNode }) {
  const qc = useQueryClient()

  // All mutable state lives in refs — the provider never re-renders on its own.
  const entries     = useRef<Map<string, RunEntry>>(new Map())
  const subCounts   = useRef<Map<string, number>>(new Map())
  const sseCleanups = useRef<Map<string, () => void>>(new Map())
  const listeners   = useRef<Map<string, Set<() => void>>>(new Map())

  function notify(runID: string) {
    listeners.current.get(runID)?.forEach((fn) => fn())
  }

  function subscribeToRun(runID: string, callback: () => void): () => void {
    if (!listeners.current.has(runID)) listeners.current.set(runID, new Set())
    listeners.current.get(runID)!.add(callback)
    return () => listeners.current.get(runID)?.delete(callback)
  }

  function getSnapshot(runID: string): RunSnapshot {
    return entries.current.get(runID)?.snapshot ?? EMPTY_SNAPSHOT
  }

  function openSSE(runID: string, jobID: string): () => void {
    const count = (subCounts.current.get(runID) ?? 0) + 1
    subCounts.current.set(runID, count)

    if (count === 1) {
      entries.current.set(runID, { snapshot: EMPTY_SNAPSHOT, jobId: jobID })
      let initialConnect = true

      const cleanup = openEventSource(`/api/runs/${runID}/progress`, (es, markDone) => {
        // On reconnect, clear stale events so transfers completed while
        // disconnected fall back to REST data.
        if (!initialConnect) {
          const entry = entries.current.get(runID)
          if (entry) {
            entry.snapshot = { ...entry.snapshot, events: new Map() }
            notify(runID)
          }
        }
        initialConnect = false

        es.onmessage = (e) => {
          try {
            const ev: ProgressEvent = JSON.parse(e.data)
            const entry = entries.current.get(runID)
            if (!entry) return
            const events = new Map(entry.snapshot.events)
            events.set(ev.transfer_id, ev)
            entry.snapshot = { ...entry.snapshot, events }
            notify(runID)
            if (ev.status === 'done') {
              qc.invalidateQueries({ queryKey: ['run', runID] })
            }
          } catch { /* malformed event — ignore */ }
        }

        es.addEventListener('run_status', (e: MessageEvent) => {
          try {
            const ev = JSON.parse(e.data)
            if (ev.run_status) {
              const entry = entries.current.get(runID)
              if (!entry) return
              entry.snapshot = { ...entry.snapshot, runStatus: ev.run_status }
              notify(runID)
              if (isTerminalStatus(ev.run_status)) {
                qc.invalidateQueries({ queryKey: ['run', runID] })
                qc.invalidateQueries({ queryKey: ['runs', jobID] })
              }
            }
          } catch { /* ignore */ }
        })

        es.addEventListener('done', () => {
          const entry = entries.current.get(runID)
          if (entry) {
            entry.snapshot = { ...entry.snapshot, isDone: true }
            notify(runID)
          }
          // Handles the case where run_status was dropped but done still fired.
          qc.invalidateQueries({ queryKey: ['run', runID] })
          qc.invalidateQueries({ queryKey: ['runs', jobID] })
          markDone()
        })

        es.onerror = () => { es.close() }
      })

      sseCleanups.current.set(runID, cleanup)
    }

    return () => {
      const newCount = (subCounts.current.get(runID) ?? 1) - 1
      subCounts.current.set(runID, newCount)
      if (newCount <= 0) {
        sseCleanups.current.get(runID)?.()
        sseCleanups.current.delete(runID)
        subCounts.current.delete(runID)
        entries.current.delete(runID)
        listeners.current.delete(runID)
      }
    }
  }

  return (
    <RunStateContext.Provider value={{ subscribeToRun, getSnapshot, openSSE }}>
      {children}
    </RunStateContext.Provider>
  )
}

export function useRunStateContext() {
  const ctx = useContext(RunStateContext)
  if (!ctx) throw new Error('useRunStateContext must be used within RunStateProvider')
  return ctx
}
