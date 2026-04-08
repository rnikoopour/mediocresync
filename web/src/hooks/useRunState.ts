import { useCallback, useEffect, useSyncExternalStore } from 'react'
import type { Run } from '../api/types'
import { useRunStateContext, EMPTY_SNAPSHOT } from '../context/RunStateContext'

export interface RunState {
  // Raw SSE data
  liveEvents: import('../context/RunStateContext').RunSnapshot['events']
  runStatus: string | null
  isDone: boolean
  // Derived status — SSE runStatus takes precedence over run.status for
  // terminal states; run.status is used for 'canceling' since the SSE
  // event fires before the DB reflects the final state.
  effectiveStatus: string
  isRunning: boolean
  isCanceling: boolean
  runEnded: boolean
}

/**
 * Subscribe to live SSE state for a run and derive display-ready status flags.
 *
 * Pass `runID: null` to skip the subscription (e.g. when a row is collapsed).
 * `run` is needed to derive effectiveStatus when SSE has not yet fired.
 */
export function useRunState(runID: string | null, jobID: string, run: Run | undefined): RunState {
  const ctx = useRunStateContext()

  // Open/ref-count the SSE connection for this run.
  useEffect(() => {
    if (!runID) return
    return ctx.openSSE(runID, jobID)
  }, [runID, jobID, ctx])

  // Stable subscribe/getSnapshot callbacks for useSyncExternalStore.
  const subscribe = useCallback(
    (cb: () => void) => (runID ? ctx.subscribeToRun(runID, cb) : () => {}),
    [runID, ctx],
  )
  const getSnapshot = useCallback(
    () => (runID ? ctx.getSnapshot(runID) : EMPTY_SNAPSHOT),
    [runID, ctx],
  )

  const { events: liveEvents, runStatus, isDone } = useSyncExternalStore(subscribe, getSnapshot)

  // SSE runStatus overrides run.status once a terminal status arrives.
  // We keep run.status for 'canceling' because the SSE fires before the
  // DB transition and run.status already reflects canceling correctly.
  const effectiveStatus =
    runStatus && runStatus !== 'canceling' ? runStatus : (run?.status ?? 'running')

  const isRunning   = effectiveStatus === 'running' || effectiveStatus === 'canceling'
  const isCanceling = effectiveStatus === 'canceling'
  const runEnded    = !isRunning

  return { liveEvents, runStatus, isDone, effectiveStatus, isRunning, isCanceling, runEnded }
}
