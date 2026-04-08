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
    expect(mock.openEventSource).toHaveBeenCalledTimes(1)
    expect(mock.connections[0].url).toBe('/api/runs/run-1/progress')
  })

  it('does not open a second connection when a second subscriber joins', () => {
    const { result } = renderHook(() => useRunStateContext(), { wrapper: makeWrapper(mock.openEventSource) })
    act(() => {
      result.current.openSSE('run-1', 'job-1')
      result.current.openSSE('run-1', 'job-1')
    })
    expect(mock.openEventSource).toHaveBeenCalledTimes(1)
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
    expect(mock.connections[0].cleanup).not.toHaveBeenCalled()
    act(() => { unsub2() })
  })

  it('closes the connection when the last subscriber leaves', () => {
    const { result } = renderHook(() => useRunStateContext(), { wrapper: makeWrapper(mock.openEventSource) })
    let unsub!: () => void
    act(() => { unsub = result.current.openSSE('run-1', 'job-1') })
    act(() => { unsub() })
    expect(mock.connections[0].cleanup).toHaveBeenCalledTimes(1)
  })
})

describe('RunStateContext — snapshot updates', () => {
  let mock: ReturnType<typeof makeMockOpenEventSource>

  beforeEach(() => { mock = makeMockOpenEventSource() })

  it('starts with EMPTY_SNAPSHOT before any events', () => {
    const { result } = renderHook(() => useRunStateContext(), { wrapper: makeWrapper(mock.openEventSource) })
    act(() => { result.current.openSSE('run-1', 'job-1') })
    const snap = result.current.getSnapshot('run-1')
    expect(snap.runStatus).toBeNull()
    expect(snap.isDone).toBe(false)
    expect(snap.events.size).toBe(0)
  })

  it('updates snapshot when a progress message arrives', () => {
    const { result } = renderHook(() => useRunStateContext(), { wrapper: makeWrapper(mock.openEventSource) })
    act(() => { result.current.openSSE('run-1', 'job-1') })
    act(() => {
      mock.connections[0].es.fireMessage({
        run_id: 'run-1', transfer_id: 'tx-1', remote_path: '/a',
        size_bytes: 1000, bytes_xferred: 500, percent: 50, speed_bps: 100, status: 'in_progress',
      })
    })
    const snap = result.current.getSnapshot('run-1')
    expect(snap.events.size).toBe(1)
    expect(snap.events.get('tx-1')?.percent).toBe(50)
  })

  it('updates runStatus when a run_status event arrives', () => {
    const { result } = renderHook(() => useRunStateContext(), { wrapper: makeWrapper(mock.openEventSource) })
    act(() => { result.current.openSSE('run-1', 'job-1') })
    act(() => { mock.connections[0].es.fireEvent('run_status', { run_status: 'completed' }) })
    expect(result.current.getSnapshot('run-1').runStatus).toBe('completed')
  })

  it('sets isDone and calls markDone when done event arrives', () => {
    const { result } = renderHook(() => useRunStateContext(), { wrapper: makeWrapper(mock.openEventSource) })
    act(() => { result.current.openSSE('run-1', 'job-1') })
    act(() => { mock.connections[0].es.fireEvent('done', {}) })
    expect(result.current.getSnapshot('run-1').isDone).toBe(true)
    expect(mock.connections[0].markDone).toHaveBeenCalledTimes(1)
  })

  it('notifies subscribers when snapshot changes', () => {
    const { result } = renderHook(() => useRunStateContext(), { wrapper: makeWrapper(mock.openEventSource) })
    act(() => { result.current.openSSE('run-1', 'job-1') })

    let notified = 0
    act(() => { result.current.subscribeToRun('run-1', () => { notified++ }) })
    act(() => { mock.connections[0].es.fireEvent('run_status', { run_status: 'completed' }) })
    expect(notified).toBe(1)
  })
})
