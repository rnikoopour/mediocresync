import { NavLink, Outlet } from 'react-router-dom'

const navItems = [
  { to: '/connections', label: 'Connections' },
  { to: '/jobs',        label: 'Sync Jobs' },
]

export function Layout() {
  return (
    <div className="min-h-screen bg-gray-50 flex">
      <nav className="w-52 bg-white border-r border-gray-200 flex flex-col">
        <div className="px-4 py-5 border-b border-gray-200">
          <span className="font-semibold text-gray-900 text-sm">go-ftpes</span>
        </div>
        <ul className="flex-1 py-3 space-y-0.5">
          {navItems.map(({ to, label }) => (
            <li key={to}>
              <NavLink
                to={to}
                className={({ isActive }) =>
                  `block px-4 py-2 text-sm rounded-md mx-2 ${
                    isActive
                      ? 'bg-blue-50 text-blue-700 font-medium'
                      : 'text-gray-600 hover:bg-gray-100'
                  }`
                }
              >
                {label}
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
