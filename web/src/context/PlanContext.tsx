import { createContext, useContext, useState } from 'react'
import { api } from '../api/client'
import type { PlanResult } from '../api/types'

interface PlanEntry {
  status: 'running' | 'done' | 'error'
  result?: PlanResult
  error?: string
}

interface PlanContextValue {
  plans: Record<string, PlanEntry>
  runPlan: (jobId: string) => void
  dismissPlan: (jobId: string) => void
}

const PlanContext = createContext<PlanContextValue | null>(null)

export function PlanProvider({ children }: { children: React.ReactNode }) {
  const [plans, setPlans] = useState<Record<string, PlanEntry>>({})

  function runPlan(jobId: string) {
    setPlans((p) => ({ ...p, [jobId]: { status: 'running' } }))
    api.jobs.plan(jobId)
      .then((result) => setPlans((p) => ({ ...p, [jobId]: { status: 'done', result } })))
      .catch((err: Error) => setPlans((p) => ({ ...p, [jobId]: { status: 'error', error: err.message } })))
  }

  function dismissPlan(jobId: string) {
    setPlans((p) => {
      const next = { ...p }
      delete next[jobId]
      return next
    })
  }

  return (
    <PlanContext.Provider value={{ plans, runPlan, dismissPlan }}>
      {children}
    </PlanContext.Provider>
  )
}

export function usePlan() {
  const ctx = useContext(PlanContext)
  if (!ctx) throw new Error('usePlan must be used within PlanProvider')
  return ctx
}
