import { describe, it, expect, beforeEach } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { RunStateProvider } from '../context/RunStateContext'
import { useRunState } from './useRunState'
import { makeMockOpenEventSource } from '../test/mockEventSource'
import type { Run } from '../api/types'

function buildRun(overrides: Partial<Run> = {}): Run {
  return {
    id: 'run-1',
    job_id: 'job-1',
    status: 'running',
    started_at: '2024-01-01T00:00:00Z',
    total_files: 10,
    copied_files: 0,
    skipped_files: 0,
    failed_files: 0,
    total_size_bytes: 1_048_576,
    bytes_copied: 0,
    ...overrides,
  }
}

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

describe('useRunState — status derivation', () => {
  // Each case fires the given SSE runStatus event (or none if null) and asserts
  // the derived fields. Using non-optional connection access so a missing
  // connection causes an immediate failure rather than a vacuous pass.
  it.each([
    { sseStatus: null,        dbStatus: 'running',   effectiveStatus: 'running',   isRunning: true,  runEnded: false },
    { sseStatus: null,        dbStatus: 'canceling', effectiveStatus: 'canceling', isRunning: true,  runEnded: false },
    { sseStatus: 'completed', dbStatus: 'running',   effectiveStatus: 'completed', isRunning: false, runEnded: true  },
    { sseStatus: 'failed',    dbStatus: 'running',   effectiveStatus: 'failed',    isRunning: false, runEnded: true  },
    // SSE 'canceling' fires before the DB transition — effectiveStatus defers
    // to run.status so the component reflects the DB value, not a transient SSE one.
    { sseStatus: 'canceling', dbStatus: 'running',   effectiveStatus: 'running',   isRunning: true,  runEnded: false },
    { sseStatus: 'canceling', dbStatus: 'canceling', effectiveStatus: 'canceling', isRunning: true,  runEnded: false },
  ])('sseStatus=$sseStatus + dbStatus=$dbStatus → effectiveStatus=$effectiveStatus', ({ sseStatus, dbStatus, effectiveStatus, isRunning, runEnded }) => {
    const mock = makeMockOpenEventSource()
    const run = buildRun({ status: dbStatus as Run['status'] })
    const { result } = renderHook(
      () => useRunState('run-1', 'job-1', run),
      { wrapper: makeWrapper(mock.openEventSource) },
    )

    // Connection must be open before we can fire SSE events
    expect(mock.connections).toHaveLength(1)

    if (sseStatus) {
      act(() => { mock.connections[0].es.fireEvent('run_status', { run_status: sseStatus }) })
    }

    expect(result.current.effectiveStatus).toBe(effectiveStatus)
    expect(result.current.isRunning).toBe(isRunning)
    expect(result.current.runEnded).toBe(runEnded)
  })
})

describe('useRunState — speed calculations', () => {
  let mock: ReturnType<typeof makeMockOpenEventSource>

  beforeEach(() => { mock = makeMockOpenEventSource() })

  it('aggregates in_progress events into liveSpeedBps while running', () => {
    const run = buildRun({ status: 'running' })
    const { result } = renderHook(
      () => useRunState('run-1', 'job-1', run),
      { wrapper: makeWrapper(mock.openEventSource) },
    )
    expect(mock.connections).toHaveLength(1)
    act(() => {
      mock.connections[0].es.fireMessage({ transfer_id: 'tx-1', status: 'in_progress', speed_bps: 500, percent: 50 })
      mock.connections[0].es.fireMessage({ transfer_id: 'tx-2', status: 'in_progress', speed_bps: 300, percent: 30 })
    })
    expect(result.current.liveSpeedBps).toBe(800)
  })

  it('excludes non-in_progress events from liveSpeedBps', () => {
    const run = buildRun({ status: 'running' })
    const { result } = renderHook(
      () => useRunState('run-1', 'job-1', run),
      { wrapper: makeWrapper(mock.openEventSource) },
    )
    expect(mock.connections).toHaveLength(1)
    act(() => {
      mock.connections[0].es.fireMessage({ transfer_id: 'tx-1', status: 'done', speed_bps: 999, percent: 100 })
    })
    expect(result.current.liveSpeedBps).toBe(0)
  })

  it('returns liveSpeedBps=0 when run is not running', () => {
    const run = buildRun({ status: 'completed' })
    const { result } = renderHook(
      () => useRunState(null, 'job-1', run),
      { wrapper: makeWrapper(mock.openEventSource) },
    )
    expect(result.current.liveSpeedBps).toBe(0)
  })

  it.each([
    {
      label: 'uses transfers_started_at when present',
      run: { bytes_copied: 1_048_576, finished_at: '2024-01-01T00:00:10Z', transfers_started_at: '2024-01-01T00:00:00Z' },
      expectedBps: 1_048_576 / 10,
    },
    {
      label: 'falls back to started_at when transfers_started_at is absent',
      run: { bytes_copied: 1_048_576, started_at: '2024-01-01T00:00:00Z', finished_at: '2024-01-01T00:00:10Z', transfers_started_at: undefined },
      expectedBps: 1_048_576 / 10,
    },
  ])('avgSpeedBps: $label', ({ run: overrides, expectedBps }) => {
    const run = buildRun({ status: 'completed', ...overrides })
    const { result } = renderHook(
      () => useRunState(null, 'job-1', run),
      { wrapper: makeWrapper(mock.openEventSource) },
    )
    expect(result.current.avgSpeedBps).toBeCloseTo(expectedBps)
  })

  it('returns avgSpeedBps=null when run has no bytes_copied', () => {
    const run = buildRun({ status: 'completed', bytes_copied: 0, finished_at: '2024-01-01T00:00:10Z' })
    const { result } = renderHook(
      () => useRunState(null, 'job-1', run),
      { wrapper: makeWrapper(mock.openEventSource) },
    )
    expect(result.current.avgSpeedBps).toBeNull()
  })
})

describe('useRunState — null runID', () => {
  it('returns empty state and opens no SSE connection', () => {
    const mock = makeMockOpenEventSource()
    const run = buildRun()
    const { result } = renderHook(
      () => useRunState(null, 'job-1', run),
      { wrapper: makeWrapper(mock.openEventSource) },
    )
    expect(mock.connections).toHaveLength(0)
    expect(result.current.liveEvents.size).toBe(0)
    expect(result.current.runStatus).toBeNull()
    expect(result.current.liveSpeedBps).toBe(0)
    expect(result.current.avgSpeedBps).toBeNull()
  })
})
