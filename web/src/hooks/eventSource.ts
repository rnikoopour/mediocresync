/**
 * Opens an EventSource with automatic reconnection when the page becomes
 * visible after being hidden (e.g. phone lock/unlock in Safari).
 *
 * The native EventSource retries on transient errors, but mobile browsers
 * often drop the connection entirely during suspend/resume cycles and do
 * not reconnect. This wrapper listens for `visibilitychange` and reopens
 * the connection whenever the page comes back into the foreground.
 *
 * @param url    The SSE endpoint URL.
 * @param setup  Called each time a new EventSource is created. Attach all
 *               event listeners here. Call `markDone()` when the stream has
 *               intentionally ended (e.g. run completed, plan finished) to
 *               prevent further reconnection attempts.
 * @returns      A cleanup function — call it to permanently close the stream.
 */
export function openEventSource(
  url: string,
  setup: (es: EventSource, markDone: () => void) => void,
): () => void {
  let es: EventSource
  let done = false
  let destroyed = false

  function connect() {
    if (done || destroyed) return
    es?.close()
    es = new EventSource(url)
    setup(es, () => {
      done = true
      es.close()
    })
  }

  function onVisible() {
    if (
      document.visibilityState === 'visible' &&
      !done &&
      (!es || es.readyState === EventSource.CLOSED)
    ) {
      connect()
    }
  }

  connect()
  document.addEventListener('visibilitychange', onVisible)

  return () => {
    destroyed = true
    es?.close()
    document.removeEventListener('visibilitychange', onVisible)
  }
}
