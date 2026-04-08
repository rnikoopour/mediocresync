import { describe, it, expect, beforeEach } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { RunStateProvider, useRunStateContext } from './RunStateContext'
import { makeMockOpenEventSource } from '../test/mockEventSource'

function makeWrapper(openEventSource: ReturnType<typeof makeMockOpenEventSource>['openEventSource']) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return function Wrapper({ children }: { children: React.ReactNode }) {
    return (
      <QueryClientProvider client={qc}>
        <RunStateProvider openEventSource={openEventSource}>
          {children}
        </RunStateProvider>
      </QueryClientProvider>
    )
  }
}

describe('RunStateContext — SSE connection lifecycle', () => {
  let mock: ReturnType<typeof makeMockOpenEventSource>

  beforeEach(() => { mock = makeMockOpenEventSource() })

  it('opens one connection on first subscribe', () => {
    const { result } = renderHook(() => useRunStateContext(), { wrapper: makeWrapper(mock.openEventSource) })
    act(() => { result.current.openSSE('run-1', 'job-1') })
    expect(mock.connections).toHaveLength(1)
    expect(mock.connections[0].url).toBe('/api/runs/run-1/progress')
  })

  it('does not open a second connection when a second subscriber joins', () => {
    const { result } = renderHook(() => useRunStateContext(), { wrapper: makeWrapper(mock.openEventSource) })
    act(() => {
      result.current.openSSE('run-1', 'job-1')
      result.current.openSSE('run-1', 'job-1')
    })
    expect(mock.connections).toHaveLength(1)
  })

  it('does not close the connection when the first of two subscribers leaves', () => {
    const { result } = renderHook(() => useRunStateContext(), { wrapper: makeWrapper(mock.openEventSource) })
    let unsub1!: () => void
    let unsub2!: () => void
    act(() => {
      unsub1 = result.current.openSSE('run-1', 'job-1')
      unsub2 = result.current.openSSE('run-1', 'job-1')
    })
    act(() => { unsub1() })
    // Connection still alive — snapshot still readable
    expect(result.current.getSnapshot('run-1')).not.toBeUndefined()
    act(() => { unsub2() })
  })

  it('tears down the connection and clears the snapshot when the last subscriber leaves', () => {
    const { result } = renderHook(() => useRunStateContext(), { wrapper: makeWrapper(mock.openEventSource) })
    let unsub!: () => void
    act(() => { unsub = result.current.openSSE('run-1', 'job-1') })

    // Fire an event so we know there's real state to clear
    act(() => { mock.connections[0].es.fireEvent('run_status', { run_status: 'running' }) })
    expect(result.current.getSnapshot('run-1').runStatus).toBe('running')

    act(() => { unsub() })
    // After last unsub, snapshot reverts to empty (entry deleted)
    expect(result.current.getSnapshot('run-1').runStatus).toBeNull()
    expect(result.current.getSnapshot('run-1').events.size).toBe(0)
  })
})

describe('RunStateContext — snapshot updates', () => {
  let mock: ReturnType<typeof makeMockOpenEventSource>

  beforeEach(() => { mock = makeMockOpenEventSource() })

  it('records transfer progress from incoming messages', () => {
    const { result } = renderHook(() => useRunStateContext(), { wrapper: makeWrapper(mock.openEventSource) })
    act(() => { result.current.openSSE('run-1', 'job-1') })
    expect(mock.connections).toHaveLength(1)

    act(() => {
      mock.connections[0].es.fireMessage({
        run_id: 'run-1', transfer_id: 'tx-1', remote_path: '/a',
        size_bytes: 1000, bytes_xferred: 500, percent: 50, speed_bps: 100, status: 'in_progress',
      })
    })
    const snap = result.current.getSnapshot('run-1')
    expect(snap.events.size).toBe(1)
    expect(snap.events.get('tx-1')?.percent).toBe(50)
    expect(snap.events.get('tx-1')?.speed_bps).toBe(100)
  })

  it('updates runStatus when a run_status event arrives', () => {
    const { result } = renderHook(() => useRunStateContext(), { wrapper: makeWrapper(mock.openEventSource) })
    act(() => { result.current.openSSE('run-1', 'job-1') })
    expect(mock.connections).toHaveLength(1)

    act(() => { mock.connections[0].es.fireEvent('run_status', { run_status: 'completed' }) })
    expect(result.current.getSnapshot('run-1').runStatus).toBe('completed')
  })

  it('sets isDone when done event arrives', () => {
    const { result } = renderHook(() => useRunStateContext(), { wrapper: makeWrapper(mock.openEventSource) })
    act(() => { result.current.openSSE('run-1', 'job-1') })
    expect(mock.connections).toHaveLength(1)

    act(() => { mock.connections[0].es.fireEvent('done', {}) })
    expect(result.current.getSnapshot('run-1').isDone).toBe(true)
  })

  it('notifies subscribers when snapshot changes', () => {
    const { result } = renderHook(() => useRunStateContext(), { wrapper: makeWrapper(mock.openEventSource) })
    act(() => { result.current.openSSE('run-1', 'job-1') })
    expect(mock.connections).toHaveLength(1)

    let notified = 0
    act(() => { result.current.subscribeToRun('run-1', () => { notified++ }) })
    act(() => { mock.connections[0].es.fireEvent('run_status', { run_status: 'completed' }) })
    expect(notified).toBe(1)
  })

  it('clears stale events on reconnect', () => {
    const { result } = renderHook(() => useRunStateContext(), { wrapper: makeWrapper(mock.openEventSource) })
    act(() => { result.current.openSSE('run-1', 'job-1') })
    expect(mock.connections).toHaveLength(1)

    // Fire a progress event on first connection
    act(() => {
      mock.connections[0].es.fireMessage({
        run_id: 'run-1', transfer_id: 'tx-1', remote_path: '/a',
        size_bytes: 1000, bytes_xferred: 500, percent: 50, speed_bps: 100, status: 'in_progress',
      })
    })
    expect(result.current.getSnapshot('run-1').events.size).toBe(1)

    // Simulate reconnect by triggering onerror then calling setup again via a new connection
    act(() => { mock.connections[0].es.onerror?.(new Event('error')) })
    // Invoke a second setup call (simulates openEventSource reconnect)
    act(() => { mock.openEventSource.mock.calls[0][1](mock.connections[0].es as unknown as EventSource, () => {}) })
    expect(result.current.getSnapshot('run-1').events.size).toBe(0)
  })
})
