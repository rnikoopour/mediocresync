import { createContext, useContext, useRef, useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
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

export type JobStatus = 'idle' | 'planning' | 'running'
type JobEventHandler = (ev: Record<string, unknown>) => void

interface PlanContextValue {
  plans: Record<string, PlanEntry>
  jobStatuses: Record<string, JobStatus>
  runPlan: (jobId: string) => void
  subscribePlan: (jobId: string) => () => void
  subscribeJobEvents: (jobId: string) => () => void
  onJobEvent: (jobId: string, handler: JobEventHandler) => () => void
  dismissPlan: (jobId: string) => void
  unskipFile: (jobId: string, remotePath: string) => void
  skipFile: (jobId: string, remotePath: string) => void
}

const PlanContext = createContext<PlanContextValue | null>(null)

export function PlanProvider({ children }: { children: React.ReactNode }) {
  const qc = useQueryClient()
  const [plans, setPlans] = useState<Record<string, PlanEntry>>({})
  const [jobStatuses, setJobStatuses] = useState<Record<string, JobStatus>>({})

  // Plan SSE subscriptions — one EventSource cleanup per job.
  const planSubs = useRef<Map<string, () => void>>(new Map())

  // Job events SSE — ref-counted so multiple components can subscribe to the
  // same job without opening duplicate connections.
  const jobSubCounts = useRef<Map<string, number>>(new Map())
  const jobSubCleanups = useRef<Map<string, () => void>>(new Map())

  // Per-job event handlers for page-specific events (e.g. plan_file_updated).
  const jobEventHandlers = useRef<Map<string, Set<JobEventHandler>>>(new Map())

  // ── Plan events ─────────────────────────────────────────────────────────────

  function connectPlanEvents(jobId: string): () => void {
    planSubs.current.get(jobId)?.()

    const cleanup = openEventSource(`/api/jobs/${jobId}/plan/events`, (es) => {
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

      es.onerror = () => { es.close() }
    })

    planSubs.current.set(jobId, cleanup)
    return () => {
      cleanup()
      planSubs.current.delete(jobId)
    }
  }

  function runPlan(jobId: string) {
    setPlans((p) => ({ ...p, [jobId]: { status: 'running', scannedFiles: 0, scannedFolders: 0 } }))

    fetch(`/api/jobs/${jobId}/plan`, { method: 'POST' })
      .then((res) => {
        if (!res.ok && res.status !== 409) throw new Error('failed to start plan')
      })
      .catch((err: Error) => {
        setPlans((p) => ({ ...p, [jobId]: { status: 'error', scannedFiles: 0, scannedFolders: 0, error: err.message } }))
      })
  }

  function subscribePlan(jobId: string): () => void {
    return connectPlanEvents(jobId)
  }

  // ── Job events ───────────────────────────────────────────────────────────────

  function openJobConnection(jobId: string) {
    const cleanup = openEventSource(`/api/jobs/${jobId}/events`, (es) => {
      // On (re)connect, refresh run list to catch any events missed while
      // the connection was down.
      es.onopen = () => {
        qc.invalidateQueries({ queryKey: ['runs', jobId] })
      }
      es.onmessage = (e) => {
        try {
          const ev = JSON.parse(e.data) as Record<string, unknown>
          const status = ev.status as string
          if (status === 'planning') {
            setJobStatuses((s) => ({ ...s, [jobId]: 'planning' }))
          } else if (status === 'started') {
            setJobStatuses((s) => ({ ...s, [jobId]: 'running' }))
            qc.invalidateQueries({ queryKey: ['runs', jobId] })
          } else if (status === 'run_finished' || status === 'runs_pruned') {
            setJobStatuses((s) => ({ ...s, [jobId]: 'idle' }))
            qc.invalidateQueries({ queryKey: ['runs', jobId] })
          }
          // Fan out to any page-specific handlers (e.g. plan_file_updated).
          jobEventHandlers.current.get(jobId)?.forEach((h) => h(ev))
        } catch {
          // malformed event — ignore
        }
      }
    })
    jobSubCleanups.current.set(jobId, cleanup)
  }

  function subscribeJobEvents(jobId: string): () => void {
    const count = (jobSubCounts.current.get(jobId) ?? 0) + 1
    jobSubCounts.current.set(jobId, count)
    if (count === 1) openJobConnection(jobId)

    return () => {
      const newCount = (jobSubCounts.current.get(jobId) ?? 1) - 1
      jobSubCounts.current.set(jobId, newCount)
      if (newCount <= 0) {
        jobSubCleanups.current.get(jobId)?.()
        jobSubCleanups.current.delete(jobId)
        jobSubCounts.current.delete(jobId)
        setJobStatuses((s) => {
          const { [jobId]: _, ...rest } = s
          return rest
        })
      }
    }
  }

  function onJobEvent(jobId: string, handler: JobEventHandler): () => void {
    if (!jobEventHandlers.current.has(jobId)) {
      jobEventHandlers.current.set(jobId, new Set())
    }
    jobEventHandlers.current.get(jobId)!.add(handler)
    return () => {
      jobEventHandlers.current.get(jobId)?.delete(handler)
    }
  }

  // ── Plan file state ──────────────────────────────────────────────────────────

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
      const toSkip = files.filter((f) => f.action === 'skip').length
      return { ...p, [jobId]: { ...entry, result: { ...entry.result, files, to_copy: toCopy, to_skip: toSkip } } }
    })
  }

  function dismissPlan(jobId: string) {
    api.jobs.dismissPlan(jobId).catch(() => {})
    setPlans((p) => {
      const next = { ...p }
      delete next[jobId]
      return next
    })
  }

  return (
    <PlanContext.Provider value={{ plans, jobStatuses, runPlan, subscribePlan, subscribeJobEvents, onJobEvent, dismissPlan, unskipFile, skipFile }}>
      {children}
    </PlanContext.Provider>
  )
}

export function usePlan() {
  const ctx = useContext(PlanContext)
  if (!ctx) throw new Error('usePlan must be used within PlanProvider')
  return ctx
}
