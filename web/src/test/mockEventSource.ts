import { vi } from 'vitest'

/**
 * A controllable fake EventSource for testing SSE-driven code.
 * Lets tests fire messages and named events programmatically.
 */
export class FakeEventSource {
  onmessage: ((e: MessageEvent) => void) | null = null
  onerror:   ((e: Event) => void)         | null = null
  readyState = 1 // OPEN

  private listeners = new Map<string, Array<(e: MessageEvent) => void>>()

  addEventListener(type: string, listener: (e: MessageEvent) => void) {
    if (!this.listeners.has(type)) this.listeners.set(type, [])
    this.listeners.get(type)!.push(listener)
  }

  removeEventListener(type: string, listener: (e: MessageEvent) => void) {
    this.listeners.set(type, (this.listeners.get(type) ?? []).filter((l) => l !== listener))
  }

  close() { this.readyState = 2 }

  // ── test helpers ─────────────────────────────────────────────────────────────

  fireMessage(data: unknown) {
    this.onmessage?.(new MessageEvent('message', { data: JSON.stringify(data) }))
  }

  fireEvent(type: string, data: unknown) {
    const event = new MessageEvent(type, { data: JSON.stringify(data) })
    this.listeners.get(type)?.forEach((l) => l(event))
  }
}

/**
 * Returns a mock openEventSource function and an array of connections opened
 * so far. Each connection exposes the FakeEventSource and a markDone spy.
 */
export function makeMockOpenEventSource() {
  const connections: Array<{ url: string; es: FakeEventSource; markDone: () => void; cleanup: ReturnType<typeof vi.fn> }> = []

  const openEventSource = vi.fn((url: string, setup: (es: EventSource, markDone: () => void) => void) => {
    const es = new FakeEventSource()
    const markDone = vi.fn()
    const cleanup = vi.fn()
    setup(es as unknown as EventSource, markDone)
    connections.push({ url, es, markDone, cleanup })
    return cleanup
  })

  return { openEventSource, connections }
}
