export function formatDateTime(date: Date | string, use24h: boolean): string {
  const d = typeof date === 'string' ? new Date(date) : date
  return d.toLocaleString(undefined, { hour12: !use24h })
}

export function formatTime(date: Date | string, use24h: boolean): string {
  const d = typeof date === 'string' ? new Date(date) : date
  return d.toLocaleTimeString(undefined, { hour12: !use24h })
}
