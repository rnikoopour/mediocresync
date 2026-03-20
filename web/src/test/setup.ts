import '@testing-library/jest-dom'

// happy-dom does not implement EventSource; provide a no-op stub so that
// components which open SSE connections don't throw during tests.
class MockEventSource {
  static CONNECTING = 0
  static OPEN       = 1
  static CLOSED     = 2

  readyState = MockEventSource.OPEN
  onmessage: ((e: MessageEvent) => void) | null = null
  onerror:   ((e: Event) => void) | null = null
  onopen:    ((e: Event) => void) | null = null

  private listeners: Map<string, EventListener[]> = new Map()

  readonly url: string
  constructor(url: string) { this.url = url }

  addEventListener(type: string, listener: EventListener) {
    const list = this.listeners.get(type) ?? []
    list.push(listener)
    this.listeners.set(type, list)
  }

  removeEventListener(type: string, listener: EventListener) {
    const list = this.listeners.get(type) ?? []
    this.listeners.set(type, list.filter((l) => l !== listener))
  }

  close() {
    this.readyState = MockEventSource.CLOSED
  }
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
;(globalThis as any).EventSource = MockEventSource
