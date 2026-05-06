import { NavLink, Outlet } from 'react-router-dom'
import { useState } from 'react'
import {
  LayoutDashboard,
  GitBranch,
  Inbox,
  Bot,
  ListOrdered,
  BarChart3,
  Menu,
  X,
  Terminal,
  LogOut,
} from 'lucide-react'
import clsx from 'clsx'
import { useAuth } from '../context/AuthContext'

const navItems = [
  { to: '/', icon: LayoutDashboard, label: 'Dashboard' },
  { to: '/stacks', icon: GitBranch, label: 'Stacks' },
  { to: '/inbox', icon: Inbox, label: 'PR Inbox' },
  { to: '/reviews', icon: Bot, label: 'AI Reviews' },
  { to: '/merge-queue', icon: ListOrdered, label: 'Merge Queue' },
  { to: '/insights', icon: BarChart3, label: 'Insights' },
]

export default function Layout() {
  const [sidebarOpen, setSidebarOpen] = useState(false)
  const { user, logout } = useAuth()

  return (
    <div className="flex h-screen overflow-hidden">
      {sidebarOpen && (
        <div
          className="fixed inset-0 z-30 bg-black/50 lg:hidden"
          onClick={() => setSidebarOpen(false)}
        />
      )}

      <aside
        className={clsx(
          'fixed inset-y-0 left-0 z-40 flex w-64 flex-col border-r border-gray-800 bg-gray-950 transition-transform lg:static lg:translate-x-0',
          sidebarOpen ? 'translate-x-0' : '-translate-x-full'
        )}
      >
        <div className="flex h-16 items-center gap-3 border-b border-gray-800 px-6">
          <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-brand-600">
            <Terminal className="h-4 w-4 text-white" />
          </div>
          <span className="text-lg font-bold tracking-tight">mgt</span>
          <span className="ml-1 rounded-md bg-gray-800 px-2 py-0.5 text-xs font-medium text-gray-400">
            beta
          </span>
        </div>

        <nav className="flex-1 space-y-1 p-4">
          {navItems.map(({ to, icon: Icon, label }) => (
            <NavLink
              key={to}
              to={to}
              end={to === '/'}
              onClick={() => setSidebarOpen(false)}
              className={({ isActive }) =>
                clsx(
                  'flex items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-medium transition-colors',
                  isActive
                    ? 'bg-brand-600/15 text-brand-400'
                    : 'text-gray-400 hover:bg-gray-800/60 hover:text-gray-200'
                )
              }
            >
              <Icon className="h-4.5 w-4.5" />
              {label}
            </NavLink>
          ))}
        </nav>

        <div className="border-t border-gray-800 p-4 space-y-3">
          {user && (
            <div className="flex items-center gap-3 rounded-lg bg-gray-900 p-3">
              <img
                src={user.avatar_url}
                alt={user.login}
                className="h-8 w-8 rounded-full ring-2 ring-gray-700"
              />
              <div className="min-w-0 flex-1">
                <p className="truncate text-sm font-medium text-gray-200">{user.login}</p>
                <p className="truncate text-xs text-gray-500">mgt-be</p>
              </div>
              <button
                onClick={logout}
                title="Sign out"
                className="rounded-md p-1.5 text-gray-500 transition-colors hover:bg-gray-800 hover:text-gray-300"
              >
                <LogOut className="h-4 w-4" />
              </button>
            </div>
          )}
          <div className="rounded-lg bg-gray-900/60 p-3">
            <p className="text-xs font-medium text-gray-400">mgt CLI</p>
            <p className="mt-1 text-xs text-gray-500">
              Authenticate with <code className="text-brand-400">mgt login</code>
            </p>
          </div>
        </div>
      </aside>

      <div className="flex flex-1 flex-col overflow-hidden">
        <header className="flex h-16 items-center gap-4 border-b border-gray-800 px-6 lg:hidden">
          <button
            onClick={() => setSidebarOpen(true)}
            className="rounded-lg p-2 text-gray-400 hover:bg-gray-800 hover:text-gray-200"
          >
            {sidebarOpen ? <X className="h-5 w-5" /> : <Menu className="h-5 w-5" />}
          </button>
          <div className="flex items-center gap-2">
            <div className="flex h-7 w-7 items-center justify-center rounded-lg bg-brand-600">
              <Terminal className="h-3.5 w-3.5 text-white" />
            </div>
            <span className="font-bold">mgt</span>
          </div>
          {user && (
            <img
              src={user.avatar_url}
              alt={user.login}
              className="ml-auto h-7 w-7 rounded-full ring-2 ring-gray-700"
            />
          )}
        </header>

        <main className="flex-1 overflow-y-auto">
          <Outlet />
        </main>
      </div>
    </div>
  )
}
