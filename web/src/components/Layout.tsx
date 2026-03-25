import { useState, useEffect } from 'react'
import { NavLink, Outlet, useMatch, useLocation } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { api } from '../api/client'
import { useDarkMode } from '../hooks/useDarkMode'
import { usePlan } from '../context/PlanContext'

export function Layout() {
  const jobsActive = useMatch('/jobs/*')
  const [dark, toggleDark] = useDarkMode()
  const { plans } = usePlan()
  const [sidebarOpen, setSidebarOpen] = useState(false)
  const location = useLocation()
  const settingsExpanded = location.pathname.startsWith('/settings')

  useEffect(() => {
    setSidebarOpen(false)
  }, [location.pathname])

  const { data: jobs = [] } = useQuery({
    queryKey: ['jobs'],
    queryFn: api.jobs.list,
  })

  const { data: versionData } = useQuery({
    queryKey: ['version'],
    queryFn: api.version.get,
    staleTime: Infinity,
  })

  return (
    <div className="min-h-screen bg-gray-50 dark:bg-gray-900 flex flex-col sm:flex-row">
      {/* Mobile topbar */}
      <div className="sm:hidden h-14 bg-white dark:bg-gray-800 border-b border-gray-200 dark:border-gray-700 flex items-center px-4 shrink-0">
        <button
          onClick={() => setSidebarOpen(true)}
          className="p-2 text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200"
          aria-label="Open menu"
        >
          <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 6h16M4 12h16M4 18h16" />
          </svg>
        </button>
        <span className="flex-1 text-center font-semibold text-gray-900 dark:text-gray-100 text-sm">MediocreSync</span>
        <button
          onClick={toggleDark}
          className="p-2 text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200 text-sm"
          aria-label="Toggle dark mode"
        >
          {dark ? '☀' : '☾'}
        </button>
      </div>

      {/* Backdrop */}
      <div
        className={`fixed inset-0 z-40 bg-black/50 sm:hidden transition-opacity duration-200 ${sidebarOpen ? 'opacity-100 pointer-events-auto' : 'opacity-0 pointer-events-none'}`}
        onClick={() => setSidebarOpen(false)}
      />

      {/* Sidebar */}
      <nav className={`fixed inset-y-0 left-0 z-50 w-52 bg-white dark:bg-gray-800 border-r border-gray-200 dark:border-gray-700 flex flex-col transition-transform duration-200 ease-in-out sm:sticky sm:top-0 sm:h-screen sm:inset-auto sm:translate-x-0 ${sidebarOpen ? 'translate-x-0' : '-translate-x-full'}`}>
        <div className="hidden sm:block px-4 py-5 border-b border-gray-200 dark:border-gray-700">
          <span className="font-semibold text-gray-900 dark:text-gray-100 text-sm">MediocreSync</span>
        </div>
        <ul className="flex-1 py-3 space-y-0.5 overflow-y-auto">
          <li>
            <NavLink
              to="/sources"
              className={({ isActive }) =>
                `block px-4 py-2 text-sm rounded-md mx-2 ${
                  isActive
                    ? 'bg-blue-50 dark:bg-gray-700 text-blue-700 dark:text-gray-100 font-medium'
                    : 'text-gray-600 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-700'
                }`
              }
            >
              Sources
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
          <li>
            <NavLink
              to="/settings"
              end
              className={({ isActive }) =>
                `block px-4 py-2 text-sm rounded-md mx-2 ${
                  isActive
                    ? 'bg-blue-50 dark:bg-gray-700 text-blue-700 dark:text-gray-100 font-medium'
                    : 'text-gray-600 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-700'
                }`
              }
            >
              Settings
            </NavLink>
          </li>
          {settingsExpanded && (
            <>
              <li>
                <NavLink
                  to="/settings/general"
                  className={({ isActive }) =>
                    `block pl-7 pr-4 py-1.5 text-xs rounded-md mx-2 ${
                      isActive
                        ? 'bg-blue-50 dark:bg-gray-700 text-blue-700 dark:text-gray-100 font-medium'
                        : 'text-gray-500 dark:text-gray-500 hover:bg-gray-100 dark:hover:bg-gray-700'
                    }`
                  }
                >
                  General
                </NavLink>
              </li>
              <li>
                <NavLink
                  to="/settings/logs"
                  className={({ isActive }) =>
                    `block pl-7 pr-4 py-1.5 text-xs rounded-md mx-2 ${
                      isActive
                        ? 'bg-blue-50 dark:bg-gray-700 text-blue-700 dark:text-gray-100 font-medium'
                        : 'text-gray-500 dark:text-gray-500 hover:bg-gray-100 dark:hover:bg-gray-700'
                    }`
                  }
                >
                  Logs
                </NavLink>
              </li>
            </>
          )}
        </ul>
        <div className="px-3 py-3 border-t border-gray-200 dark:border-gray-700 space-y-1">
          <button
            onClick={toggleDark}
            className="w-full flex items-center gap-2 px-3 py-2 text-xs text-gray-500 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-md transition-colors"
          >
            {dark ? '☀ Light mode' : '☾ Dark mode'}
          </button>
          <button
            onClick={async () => { await api.auth.logout(); window.location.href = '/login' }}
            className="w-full flex items-center gap-2 px-3 py-2 text-xs text-gray-500 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-md transition-colors"
          >
            Sign out
          </button>
          {versionData && (
            <p className="px-3 py-1 text-xs text-gray-400 dark:text-gray-600">{versionData.version}</p>
          )}
        </div>
      </nav>

      <main className="flex-1 p-4 sm:p-8 overflow-auto">
        <Outlet />
      </main>
    </div>
  )
}
