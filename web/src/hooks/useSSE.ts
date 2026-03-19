import { useEffect, useState } from 'react'
import type { ProgressEvent } from '../api/types'
import { openEventSource } from './eventSource'

export interface SSEResult {
  events: Map<string, ProgressEvent>
  runStatus: string | null
}

// Returns live transfer events and the final run status once emitted.
// Closes the EventSource when the component unmounts or runID becomes null.
// Automatically reconnects after phone lock/unlock via the visibilitychange API.
export function useSSE(runID: string | null): SSEResult {
  const [events, setEvents] = useState<Map<string, ProgressEvent>>(new Map())
  const [runStatus, setRunStatus] = useState<string | null>(null)

  useEffect(() => {
    if (!runID) return
    let initialConnect = true

    return openEventSource(`/api/runs/${runID}/progress`, (es, markDone) => {
      // On reconnect (not first connect), clear stale SSE events so that
      // transfers which completed while disconnected fall back to REST data.
      if (!initialConnect) setEvents(new Map())
      initialConnect = false

      es.onmessage = (e) => {
        try {
          const ev: ProgressEvent = JSON.parse(e.data)
          setEvents((prev) => new Map(prev).set(ev.transfer_id, ev))
        } catch {
          // malformed event — ignore
        }
      }

      es.addEventListener('run_status', (e: MessageEvent) => {
        try {
          const ev = JSON.parse(e.data)
          if (ev.run_status) setRunStatus(ev.run_status)
        } catch {
          // ignore
        }
      })

      es.addEventListener('done', () => {
        markDone() // run finished — don't reconnect
      })

      es.onerror = () => {
        es.close() // visibilitychange will reconnect when page is foregrounded
      }
    })
  }, [runID])

  // Reset state when switching runs
  useEffect(() => {
    setEvents(new Map())
    setRunStatus(null)
  }, [runID])

  return { events, runStatus }
}
