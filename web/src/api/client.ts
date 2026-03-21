import type {
  Connection, ConnectionRequest,
  SyncJob, JobRequest,
  Run, TestResult, PlanResult, BrowseEntry,
  ServerSettings, LogLevel,
} from './types'

async function request<T>(method: string, path: string, body?: unknown, throw401 = false): Promise<T> {
  const res = await fetch(`/api${path}`, {
    method,
    headers: body ? { 'Content-Type': 'application/json' } : {},
    body: body ? JSON.stringify(body) : undefined,
  })
  if (res.status === 401 && !throw401) {
    window.location.href = '/login'
    return undefined as T
  }
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(err.error ?? res.statusText)
  }
  if (res.status === 204 || res.status === 202 || res.status === 200 && res.headers.get('content-length') === '0') {
    return undefined as T
  }
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
    testDirect: (body: ConnectionRequest & { fallback_id?: string }) => request<TestResult>('POST', '/connections/test', body),
    browse: (id: string, path: string) => request<BrowseEntry[]>('GET', `/connections/${id}/browse?path=${encodeURIComponent(path)}`),
  },

  jobs: {
    list: () => request<SyncJob[]>('GET', '/jobs'),
    create: (body: JobRequest) => request<SyncJob>('POST', '/jobs', body),
    get: (id: string) => request<SyncJob>('GET', `/jobs/${id}`),
    update: (id: string, body: Partial<JobRequest>) => request<SyncJob>('PUT', `/jobs/${id}`, body),
    delete: (id: string) => request<void>('DELETE', `/jobs/${id}`),
    run: (id: string) => request<void>('POST', `/jobs/${id}/run`),
    planThenRun: (id: string) => request<void>('POST', `/jobs/${id}/planthenrun`),
    cancel: (id: string) => request<void>('DELETE', `/jobs/${id}/run`),
    plan: (id: string) => request<PlanResult>('POST', `/jobs/${id}/plan`),
    dismissPlan: (id: string) => request<void>('DELETE', `/jobs/${id}/plan`),
    deleteFileState: (id: string, path: string) => request<void>('DELETE', `/jobs/${id}/files?path=${encodeURIComponent(path)}`),
    skipFile: (id: string, path: string, sizeBytes: number, mtime: string) =>
      request<void>('PUT', `/jobs/${id}/files`, { path, size_bytes: sizeBytes, mtime }),
    listRuns: (id: string) => request<Run[]>('GET', `/jobs/${id}/runs`),
  },

  runs: {
    get: (id: string) => request<Run>('GET', `/runs/${id}`),
  },

  local: {
    browse: (path: string) => request<BrowseEntry[]>('GET', `/browse/local?path=${encodeURIComponent(path)}`),
  },

  settings: {
    get: () => request<ServerSettings>('GET', '/settings'),
    setLogLevel: (log_level: LogLevel) => request<void>('PUT', '/settings', { log_level }),
  },

  auth: {
    setup: (body: { username: string; password: string; password_confirm: string }) =>
      request<void>('POST', '/auth/setup', body),
    login: (body: { username: string; password: string }) =>
      request<void>('POST', '/auth/login', body, true),
    logout: () => request<void>('POST', '/auth/logout'),
    me: () => request<{ username: string }>('GET', '/auth/me'),
    updateCredentials: (body: { current_password: string; username?: string; new_password?: string }) =>
      request<void>('PUT', '/auth/credentials', body, true),
  },
}
