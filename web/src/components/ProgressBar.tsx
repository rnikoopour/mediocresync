interface Props {
  percent: number   // 0–100
  label?: string    // overrides the percentage text
  variant?: 'default' | 'failed' | 'not_copied' | 'loading'
}

export function ProgressBar({ percent, label, variant = 'default' }: Props) {
  if (variant === 'loading') {
    return (
      <div className="relative bg-gray-200 dark:bg-gray-600 rounded h-5 overflow-hidden">
        <div className="absolute inset-y-0 left-0 w-1/3 bg-blue-500 [animation:slide-indeterminate_1.2s_ease-in-out_infinite]" />
        <span className="absolute inset-0 flex items-center justify-center text-xs font-medium text-white">
          —
        </span>
      </div>
    )
  }
  const pct = Math.min(100, Math.max(0, percent))
  const fillColor = variant === 'failed' ? 'bg-red-500' : variant === 'not_copied' ? 'bg-gray-400 dark:bg-gray-500' : 'bg-blue-500'
  return (
    <div className="relative bg-gray-200 dark:bg-gray-600 rounded h-5 overflow-hidden">
      <div
        className={`absolute inset-y-0 left-0 ${fillColor} transition-all duration-200`}
        style={{ width: `${pct}%` }}
      />
      <span className="absolute inset-0 flex items-center justify-center text-xs font-medium text-white">
        {label ?? `${Math.round(pct)}%`}
      </span>
    </div>
  )
}
