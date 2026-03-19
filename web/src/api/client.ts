import type {
  Connection, ConnectionRequest,
  SyncJob, JobRequest,
  Run, TestResult, PlanResult, BrowseEntry,
} from './types'

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(`/api${path}`, {
    method,
    headers: body ? { 'Content-Type': 'application/json' } : {},
    body: body ? JSON.stringify(body) : undefined,
  })
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(err.error ?? res.statusText)
  }
  if (res.status === 204) return undefined as T
  return res.json()
}

// Connections
export const api = {
  connections: {
    list: () => request<Connection[]>('GET', '/connections'),
    create: (body: ConnectionRequest) => request<Connection>('POST', '/connections', body),
    get: (id: string) => request<Connection>('GET', `/connections/${id}`),
    update: (id: string, body: Partial<ConnectionRequest>) => request<Connection>('PUT', `/connections/${id}`, body),
    delete: (id: string) => request<void>('DELETE', `/connections/${id}`),
    test: (id: string) => request<TestResult>('POST', `/connections/${id}/test`),
    browse: (id: string, path: string) => request<BrowseEntry[]>('GET', `/connections/${id}/browse?path=${encodeURIComponent(path)}`),
  },

  jobs: {
    list: () => request<SyncJob[]>('GET', '/jobs'),
    create: (body: JobRequest) => request<SyncJob>('POST', '/jobs', body),
    get: (id: string) => request<SyncJob>('GET', `/jobs/${id}`),
    update: (id: string, body: Partial<JobRequest>) => request<SyncJob>('PUT', `/jobs/${id}`, body),
    delete: (id: string) => request<void>('DELETE', `/jobs/${id}`),
    trigger: (id: string) => request<void>('POST', `/jobs/${id}/run`),
    plan: (id: string) => request<PlanResult>('POST', `/jobs/${id}/plan`),
    listRuns: (id: string) => request<Run[]>('GET', `/jobs/${id}/runs`),
  },

  runs: {
    get: (id: string) => request<Run>('GET', `/runs/${id}`),
  },
}
