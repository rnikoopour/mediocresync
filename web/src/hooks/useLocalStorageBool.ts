import { useState, useEffect } from 'react'

function read(key: string, defaultValue: boolean): boolean {
  const stored = localStorage.getItem(key)
  return stored !== null ? stored === 'true' : defaultValue
}

export function useLocalStorageBool(key: string, defaultValue: boolean): [boolean, (v: boolean | ((prev: boolean) => boolean)) => void] {
  const [value, setValue] = useState<boolean>(() => read(key, defaultValue))

  // Re-read when the key changes (e.g. navigating between jobs).
  useEffect(() => {
    setValue(read(key, defaultValue))
  }, [key]) // eslint-disable-line react-hooks/exhaustive-deps

  const set = (v: boolean | ((prev: boolean) => boolean)) => {
    setValue((prev) => {
      const next = typeof v === 'function' ? v(prev) : v
      localStorage.setItem(key, String(next))
      return next
    })
  }

  return [value, set]
}
