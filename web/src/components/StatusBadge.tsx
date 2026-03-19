type Status = 'running' | 'completed' | 'failed' | 'pending' | 'in_progress' | 'done' | 'skipped'

const styles: Record<Status, string> = {
  running:     'bg-blue-100 text-blue-800',
  in_progress: 'bg-blue-100 text-blue-800',
  completed:   'bg-green-100 text-green-800',
  done:        'bg-green-100 text-green-800',
  failed:      'bg-red-100 text-red-800',
  pending:     'bg-gray-100 text-gray-600',
  skipped:     'bg-yellow-100 text-yellow-800',
}

const labels: Record<Status, string> = {
  running:     'Running',
  in_progress: 'In Progress',
  completed:   'Completed',
  done:        'Done',
  failed:      'Failed',
  pending:     'Pending',
  skipped:     'Skipped',
}

export function StatusBadge({ status }: { status: string }) {
  const s = status as Status
  return (
    <span className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium ${styles[s] ?? 'bg-gray-100 text-gray-600'}`}>
      {labels[s] ?? status}
    </span>
  )
}
