type Status = 'running' | 'completed' | 'failed' | 'canceled' | 'pending' | 'in_progress' | 'done' | 'skipped'

const styles: Record<Status, string> = {
  running:     'bg-blue-100 text-blue-800 dark:bg-gray-600 dark:text-gray-100',
  in_progress: 'bg-blue-100 text-blue-800 dark:bg-gray-600 dark:text-gray-100',
  completed:   'bg-green-100 text-green-800 dark:bg-green-900/40 dark:text-green-300',
  done:        'bg-green-100 text-green-800 dark:bg-green-900/40 dark:text-green-300',
  failed:      'bg-red-100 text-red-800 dark:bg-red-900/40 dark:text-red-300',
  canceled:    'bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-300',
  pending:     'bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-300',
  skipped:     'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/40 dark:text-yellow-300',
}

const labels: Record<Status, string> = {
  running:     'Running',
  in_progress: 'In Progress',
  completed:   'Completed',
  done:        'Done',
  failed:      'Failed',
  canceled:    'Canceled',
  pending:     'Pending',
  skipped:     'Skipped',
}

export function StatusBadge({ status }: { status: string }) {
  const s = status as Status
  return (
    <span className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium ${styles[s] ?? 'bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-300'}`}>
      {labels[s] ?? status}
    </span>
  )
}
