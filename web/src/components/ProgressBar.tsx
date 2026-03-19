interface Props {
  percent: number   // 0–100
  speedBps?: number
}

function formatSpeed(bps: number): string {
  if (bps >= 1_000_000) return `${(bps / 1_000_000).toFixed(1)} MB/s`
  if (bps >= 1_000)     return `${(bps / 1_000).toFixed(1)} KB/s`
  return `${Math.round(bps)} B/s`
}

export function ProgressBar({ percent, speedBps }: Props) {
  const pct = Math.min(100, Math.max(0, percent))
  return (
    <div className="flex items-center gap-2">
      <div className="flex-1 bg-gray-200 dark:bg-gray-600 rounded-full h-2 overflow-hidden">
        <div
          className="bg-blue-500 h-2 rounded-full transition-all duration-200"
          style={{ width: `${pct}%` }}
        />
      </div>
      <span className="text-xs text-gray-500 dark:text-gray-400 w-10 text-right">{Math.round(pct)}%</span>
      {speedBps !== undefined && speedBps > 0 && (
        <span className="text-xs text-gray-400 dark:text-gray-500 w-20 text-right">{formatSpeed(speedBps)}</span>
      )}
    </div>
  )
}
