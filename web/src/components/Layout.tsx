import { NavLink, Outlet, useMatch } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { api } from '../api/client'
import { useDarkMode } from '../hooks/useDarkMode'
import { usePlan } from '../context/PlanContext'

export function Layout() {
  const jobsActive = useMatch('/jobs/*')
  const [dark, toggleDark] = useDarkMode()
  const { plans } = usePlan()

  const { data: jobs = [] } = useQuery({
    queryKey: ['jobs'],
    queryFn: api.jobs.list,
  })

  return (
    <div className="min-h-screen bg-gray-50 dark:bg-gray-900 flex">
      <nav className="w-52 bg-white dark:bg-gray-800 border-r border-gray-200 dark:border-gray-700 flex flex-col">
        <div className="px-4 py-5 border-b border-gray-200 dark:border-gray-700">
          <span className="font-semibold text-gray-900 dark:text-gray-100 text-sm">go-ftpes</span>
        </div>
        <ul className="flex-1 py-3 space-y-0.5">
          <li>
            <NavLink
              to="/connections"
              className={({ isActive }) =>
                `block px-4 py-2 text-sm rounded-md mx-2 ${
                  isActive
                    ? 'bg-blue-50 dark:bg-gray-700 text-blue-700 dark:text-gray-100 font-medium'
                    : 'text-gray-600 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-700'
                }`
              }
            >
              Connections
            </NavLink>
          </li>
          <li>
            <NavLink
              to="/jobs"
              end
              className={({ isActive }) =>
                `block px-4 py-2 text-sm rounded-md mx-2 ${
                  isActive
                    ? 'bg-blue-50 dark:bg-gray-700 text-blue-700 dark:text-gray-100 font-medium'
                    : 'text-gray-600 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-700'
                }`
              }
            >
              Sync Jobs
            </NavLink>
          </li>
          {jobsActive && jobs.map((job) => (
            <li key={job.id}>
              <NavLink
                to={`/jobs/${job.id}`}
                className={({ isActive }) =>
                  `block pl-7 pr-4 py-1.5 text-xs rounded-md mx-2 truncate ${
                    isActive
                      ? 'bg-blue-50 dark:bg-gray-700 text-blue-700 dark:text-gray-100 font-medium'
                      : 'text-gray-500 dark:text-gray-500 hover:bg-gray-100 dark:hover:bg-gray-700'
                  }`
                }
                title={job.name}
              >
                <span className="flex items-center gap-1.5 min-w-0">
                  <span className="truncate">{job.name}</span>
                  {plans[job.id]?.status === 'running' && (
                    <span className="shrink-0 w-3 h-3 border-2 border-current border-t-transparent rounded-full animate-spin opacity-60" />
                  )}
                </span>
              </NavLink>
            </li>
          ))}
        </ul>
        <div className="px-3 py-3 border-t border-gray-200 dark:border-gray-700">
          <button
            onClick={toggleDark}
            className="w-full flex items-center gap-2 px-3 py-2 text-xs text-gray-500 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-md transition-colors"
          >
            {dark ? '☀ Light mode' : '☾ Dark mode'}
          </button>
        </div>
      </nav>
      <main className="flex-1 p-8 overflow-auto">
        <Outlet />
      </main>
    </div>
  )
}
