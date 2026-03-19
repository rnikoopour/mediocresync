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
  dismissPlan: (jobId: string) => void
  unskipFile: (jobId: string, remotePath: string) => void
}

const PlanContext = createContext<PlanContextValue | null>(null)

export function PlanProvider({ children }: { children: React.ReactNode }) {
  const [plans, setPlans] = useState<Record<string, PlanEntry>>({})

  function runPlan(jobId: string) {
    setPlans((p) => ({ ...p, [jobId]: { status: 'running', scannedFiles: 0, scannedFolders: 0 } }))

    fetch(`/api/jobs/${jobId}/plan`, { method: 'POST' })
      .then(async (res) => {
        if (!res.ok || !res.body) throw new Error('failed to start plan')

        const reader = res.body.getReader()
        const decoder = new TextDecoder()
        let buf = ''

        while (true) {
          const { done, value } = await reader.read()
          if (done) break
          buf += decoder.decode(value, { stream: true })
          const chunks = buf.split('\n\n')
          buf = chunks.pop() ?? ''

          for (const chunk of chunks) {
            const line = chunk.split('\n').find((l) => l.startsWith('data: '))
            if (!line) continue
            const msg = JSON.parse(line.slice(6))

            if (msg.error) {
              setPlans((p) => ({ ...p, [jobId]: { status: 'error', scannedFiles: 0, scannedFolders: 0, error: msg.error } }))
              return
            }
            if (msg.done) {
              setPlans((p) => ({ ...p, [jobId]: { ...p[jobId], status: 'done', result: msg.result } }))
              return
            }
            setPlans((p) => ({ ...p, [jobId]: { ...p[jobId], status: 'running', scannedFiles: msg.files, scannedFolders: msg.folders } }))
          }
        }
      })
      .catch((err: Error) => {
        setPlans((p) => ({ ...p, [jobId]: { status: 'error', scannedFiles: 0, scannedFolders: 0, error: err.message } }))
      })
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
    <PlanContext.Provider value={{ plans, runPlan, dismissPlan, unskipFile }}>
      {children}
    </PlanContext.Provider>
  )
}

export function usePlan() {
  const ctx = useContext(PlanContext)
  if (!ctx) throw new Error('usePlan must be used within PlanProvider')
  return ctx
}
