import { useRef, useCallback } from 'react'

const LONG_PRESS_MS = 500

/**
 * Returns touch event handlers that fire `onLongPress(x, y)` after the user
 * holds a finger still for LONG_PRESS_MS. Movement cancels the timer so normal
 * scrolling is unaffected.
 */
export function useLongPress(onLongPress: (x: number, y: number) => void) {
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null)

  const cancel = useCallback(() => {
    if (timer.current !== null) {
      clearTimeout(timer.current)
      timer.current = null
    }
  }, [])

  const onTouchStart = useCallback((e: React.TouchEvent) => {
    const touch = e.touches[0]
    const x = touch.clientX
    const y = touch.clientY
    timer.current = setTimeout(() => {
      timer.current = null
      onLongPress(x, y)
    }, LONG_PRESS_MS)
  }, [onLongPress])

  return { onTouchStart, onTouchEnd: cancel, onTouchMove: cancel }
}
