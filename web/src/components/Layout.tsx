import { NavLink, Outlet, useMatch } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { api } from '../api/client'

export function Layout() {
  const jobsActive = useMatch('/jobs/*')

  const { data: jobs = [] } = useQuery({
    queryKey: ['jobs'],
    queryFn: api.jobs.list,
  })

  return (
    <div className="min-h-screen bg-gray-50 flex">
      <nav className="w-52 bg-white border-r border-gray-200 flex flex-col">
        <div className="px-4 py-5 border-b border-gray-200">
          <span className="font-semibold text-gray-900 text-sm">go-ftpes</span>
        </div>
        <ul className="flex-1 py-3 space-y-0.5">
          <li>
            <NavLink
              to="/connections"
              className={({ isActive }) =>
                `block px-4 py-2 text-sm rounded-md mx-2 ${
                  isActive ? 'bg-blue-50 text-blue-700 font-medium' : 'text-gray-600 hover:bg-gray-100'
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
                  isActive ? 'bg-blue-50 text-blue-700 font-medium' : 'text-gray-600 hover:bg-gray-100'
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
                    isActive ? 'bg-blue-50 text-blue-700 font-medium' : 'text-gray-500 hover:bg-gray-100'
                  }`
                }
                title={job.name}
              >
                {job.name}
              </NavLink>
            </li>
          ))}
        </ul>
      </nav>
      <main className="flex-1 p-8 overflow-auto">
        <Outlet />
      </main>
    </div>
  )
}
