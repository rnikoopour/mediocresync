import { createContext, useContext, useState } from 'react'
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
}

const PlanContext = createContext<PlanContextValue | null>(null)

export function PlanProvider({ children }: { children: React.ReactNode }) {
  const [plans, setPlans] = useState<Record<string, PlanEntry>>({})

  function connectPlanEvents(jobId: string): () => void {
    const es = new EventSource(`/api/jobs/${jobId}/plan/events`)

    es.onmessage = (e) => {
      const msg = JSON.parse(e.data)

      if (msg.error) {
        setPlans((p) => ({ ...p, [jobId]: { status: 'error', scannedFiles: 0, scannedFolders: 0, error: msg.error } }))
        es.close()
        return
      }
      if (msg.done) {
        setPlans((p) => ({ ...p, [jobId]: { ...p[jobId], status: 'done', result: msg.result } }))
        es.close()
        return
      }
      // progress event — may arrive before the plan entry exists (another client started it)
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

    return () => es.close()
  }

  function runPlan(jobId: string) {
    setPlans((p) => ({ ...p, [jobId]: { status: 'running', scannedFiles: 0, scannedFolders: 0 } }))

    fetch(`/api/jobs/${jobId}/plan`, { method: 'POST' })
      .then((res) => {
        // 409 means another client already started a plan — just subscribe to its events.
        if (!res.ok && res.status !== 409) throw new Error('failed to start plan')
        connectPlanEvents(jobId)
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

  function dismissPlan(jobId: string) {
    setPlans((p) => {
      const next = { ...p }
      delete next[jobId]
      return next
    })
  }

  return (
    <PlanContext.Provider value={{ plans, runPlan, subscribePlan, dismissPlan, unskipFile }}>
      {children}
    </PlanContext.Provider>
  )
}

export function usePlan() {
  const ctx = useContext(PlanContext)
  if (!ctx) throw new Error('usePlan must be used within PlanProvider')
  return ctx
}
