import { createContext, useContext, useRef, useState } from 'react'
import { openEventSource } from '../hooks/eventSource'
import { api } from '../api/client'
import type { PlanResult } from '../api/types'

interface PlanEntry {
  status: 'running' | 'done' | 'error'
  scannedFiles: number
  scannedFolders: number
  result?: PlanResult
  error?: string
}

interface PlanContextValue {
  plans: Record<string, PlanEntry>
  runPlan: (jobId: string) => void
  subscribePlan: (jobId: string) => () => void
  dismissPlan: (jobId: string) => void
  unskipFile: (jobId: string, remotePath: string) => void
  skipFile: (jobId: string, remotePath: string) => void
}

const PlanContext = createContext<PlanContextValue | null>(null)

export function PlanProvider({ children }: { children: React.ReactNode }) {
  const [plans, setPlans] = useState<Record<string, PlanEntry>>({})
  // One active EventSource cleanup per job — re-subscribing replaces the old one.
  const subs = useRef<Map<string, () => void>>(new Map())

  function connectPlanEvents(jobId: string): () => void {
    // Replace any existing subscription so we don't double-subscribe.
    subs.current.get(jobId)?.()

    const cleanup = openEventSource(`/api/jobs/${jobId}/plan/events`, (es) => {
      // No markDone — the connection stays open indefinitely so future plan
      // events and dismissed signals flow through the same EventSource.
      es.onmessage = (e) => {
        const msg = JSON.parse(e.data)

        if (msg.dismissed) {
          setPlans((p) => {
            const next = { ...p }
            delete next[jobId]
            return next
          })
          return
        }
        if (msg.error) {
          setPlans((p) => ({ ...p, [jobId]: { status: 'error', scannedFiles: 0, scannedFolders: 0, error: msg.error } }))
          return
        }
        if (msg.done) {
          setPlans((p) => ({ ...p, [jobId]: { ...p[jobId], status: 'done', result: msg.result } }))
          return
        }
        // progress event
        setPlans((p) => ({
          ...p,
          [jobId]: {
            ...(p[jobId] ?? { status: 'running' as const, scannedFiles: 0, scannedFolders: 0 }),
            status: 'running',
            scannedFiles: msg.files,
            scannedFolders: msg.folders,
          },
        }))
      }

      es.onerror = () => {
        es.close()
      }
    })

    subs.current.set(jobId, cleanup)
    return () => {
      cleanup()
      subs.current.delete(jobId)
    }
  }

  function runPlan(jobId: string) {
    setPlans((p) => ({ ...p, [jobId]: { status: 'running', scannedFiles: 0, scannedFolders: 0 } }))

    fetch(`/api/jobs/${jobId}/plan`, { method: 'POST' })
      .then((res) => {
        if (!res.ok && res.status !== 409) throw new Error('failed to start plan')
        // Events arrive through the already-open plan EventSource — no extra
        // subscription needed.
      })
      .catch((err: Error) => {
        setPlans((p) => ({ ...p, [jobId]: { status: 'error', scannedFiles: 0, scannedFolders: 0, error: err.message } }))
      })
  }

  function subscribePlan(jobId: string): () => void {
    return connectPlanEvents(jobId)
  }

  function unskipFile(jobId: string, remotePath: string) {
    setPlans((p) => {
      const entry = p[jobId]
      if (!entry?.result) return p
      const files = entry.result.files.map((f) =>
        f.remote_path === remotePath ? { ...f, action: 'copy' as const } : f
      )
      const toCopy = files.filter((f) => f.action === 'copy').length
      const toSkip = files.filter((f) => f.action === 'skip').length
      return { ...p, [jobId]: { ...entry, result: { ...entry.result, files, to_copy: toCopy, to_skip: toSkip } } }
    })
  }

  function skipFile(jobId: string, remotePath: string) {
    setPlans((p) => {
      const entry = p[jobId]
      if (!entry?.result) return p
      const files = entry.result.files.map((f) =>
        f.remote_path === remotePath ? { ...f, action: 'skip' as const } : f
      )
      const toCopy = files.filter((f) => f.action === 'copy').length
      const toSkip  = files.filter((f) => f.action === 'skip').length
      return { ...p, [jobId]: { ...entry, result: { ...entry.result, files, to_copy: toCopy, to_skip: toSkip } } }
    })
  }

  function dismissPlan(jobId: string) {
    api.jobs.dismissPlan(jobId).catch(() => {})
    // Local state cleared immediately; the dismissed event through plan SSE
    // will clear all other connected clients.
    setPlans((p) => {
      const next = { ...p }
      delete next[jobId]
      return next
    })
  }

  return (
    <PlanContext.Provider value={{ plans, runPlan, subscribePlan, dismissPlan, unskipFile, skipFile }}>
      {children}
    </PlanContext.Provider>
  )
}

export function usePlan() {
  const ctx = useContext(PlanContext)
  if (!ctx) throw new Error('usePlan must be used within PlanProvider')
  return ctx
}
