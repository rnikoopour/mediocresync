import { useEffect, useRef, useState } from 'react'
import type { ProgressEvent } from '../api/types'

// Returns a map of transferID → latest ProgressEvent, updated live while the
// run is active. Closes the EventSource when the component unmounts or runID
// becomes null.
export function useSSE(runID: string | null): Map<string, ProgressEvent> {
  const [events, setEvents] = useState<Map<string, ProgressEvent>>(new Map())
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

  // Reset event map when switching runs
  useEffect(() => {
    setEvents(new Map())
  }, [runID])

  return events
}
