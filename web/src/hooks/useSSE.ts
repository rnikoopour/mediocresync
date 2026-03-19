import { useEffect, useRef, useState } from 'react'
import type { ProgressEvent } from '../api/types'

export interface SSEResult {
  events: Map<string, ProgressEvent>
  runStatus: string | null
}

// Returns live transfer events and the final run status once emitted.
// Closes the EventSource when the component unmounts or runID becomes null.
export function useSSE(runID: string | null): SSEResult {
  const [events, setEvents] = useState<Map<string, ProgressEvent>>(new Map())
  const [runStatus, setRunStatus] = useState<string | null>(null)
  const esRef = useRef<EventSource | null>(null)

  useEffect(() => {
    if (!runID) return

    const es = new EventSource(`/api/runs/${runID}/progress`)
    esRef.current = es

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
      es.close()
    })

    es.onerror = () => {
      es.close()
    }

    return () => {
      es.close()
      esRef.current = null
    }
  }, [runID])

  // Reset state when switching runs
  useEffect(() => {
    setEvents(new Map())
    setRunStatus(null)
  }, [runID])

  return { events, runStatus }
}
