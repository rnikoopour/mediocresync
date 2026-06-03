export function TabBtn<T extends string>({ value, current, label, onTab }: { value: T; current: T; label: string; onTab: (t: T) => void }) {
  return (
    <button
      onClick={() => onTab(value)}
      className={`px-3 py-1.5 text-xs font-medium rounded transition-colors ${
        current === value
          ? 'bg-gray-200 dark:bg-gray-600 text-gray-900 dark:text-gray-100'
          : 'text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300'
      }`}
    >
      {label}
    </button>
  )
}
