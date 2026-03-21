import { Link } from 'react-router-dom'

const sections = [
  { to: '/settings/general', label: 'General', description: 'Username and password' },
  { to: '/settings/logs', label: 'Logs', description: 'Live log stream' },
]

export function SettingsIndexPage() {
  return (
    <div>
      <h1 className="text-xl font-semibold text-gray-900 dark:text-gray-100 mb-6">Settings</h1>
      <div className="space-y-2">
        {sections.map(s => (
          <Link
            key={s.to}
            to={s.to}
            className="card-link flex items-center justify-between px-4 py-3"
          >
            <div>
              <div className="text-sm font-medium text-gray-900 dark:text-gray-100">{s.label}</div>
              <div className="text-xs text-gray-500 dark:text-gray-400">{s.description}</div>
            </div>
            <svg className="w-4 h-4 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
            </svg>
          </Link>
        ))}
      </div>
    </div>
  )
}
