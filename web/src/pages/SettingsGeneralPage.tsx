import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api } from '../api/client'
import type { LogLevel } from '../api/types'
import { useLocalStorageBool } from '../hooks/useLocalStorageBool'

const logLevels: LogLevel[] = ['debug', 'info', 'warn', 'error']

export function SettingsGeneralPage() {
  const qc = useQueryClient()
  const { data: me } = useQuery({ queryKey: ['auth', 'me'], queryFn: api.auth.me })
  const { data: settings } = useQuery({ queryKey: ['settings'], queryFn: api.settings.get })
  const setLogLevel = useMutation({
    mutationFn: (level: LogLevel) => api.settings.setLogLevel(level),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['settings'] }),
  })

  const [use24h, setUse24h] = useLocalStorageBool('use24hTime', false)

  const [newUsername, setNewUsername] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [currentPassword, setCurrentPassword] = useState('')
  const [error, setError] = useState('')
  const [success, setSuccess] = useState(false)
  const [loading, setLoading] = useState(false)

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setError('')
    setSuccess(false)

    if (newPassword && newPassword !== confirmPassword) {
      setError('New passwords do not match')
      return
    }

    setLoading(true)
    try {
      await api.auth.updateCredentials({
        current_password: currentPassword,
        username: newUsername || undefined,
        new_password: newPassword || undefined,
      })
      setSuccess(true)
      setCurrentPassword('')
      setNewPassword('')
      setConfirmPassword('')
      // All sessions invalidated — redirect to login.
      setTimeout(() => { window.location.href = '/login' }, 1500)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Update failed')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div>
      <h1 className="text-xl font-semibold text-gray-900 dark:text-gray-100 mb-6">General</h1>

      <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-6">
        <h2 className="text-sm font-medium text-gray-900 dark:text-gray-100 mb-4">Credentials</h2>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
              New username
            </label>
            <input
              type="text"
              autoComplete="username"
              placeholder={me?.username ?? ''}
              value={newUsername}
              onChange={e => setNewUsername(e.target.value)}
              className="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
            <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">Leave blank to keep current username</p>
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
              New password
            </label>
            <input
              type="password"
              autoComplete="new-password"
              value={newPassword}
              onChange={e => setNewPassword(e.target.value)}
              className="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
            <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">Leave blank to keep current password</p>
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
              Confirm new password
            </label>
            <input
              type="password"
              autoComplete="new-password"
              value={confirmPassword}
              onChange={e => setConfirmPassword(e.target.value)}
              className="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
          </div>

          <div className="pt-2 border-t border-gray-200 dark:border-gray-700">
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
              Current password <span className="text-red-500">*</span>
            </label>
            <input
              type="password"
              autoComplete="current-password"
              value={currentPassword}
              onChange={e => setCurrentPassword(e.target.value)}
              className="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-blue-500"
              required
            />
          </div>

          {error && <p className="text-sm text-red-600 dark:text-red-400">{error}</p>}
          {success && <p className="text-sm text-green-600 dark:text-green-400">Credentials updated. Redirecting to login…</p>}

          <button
            type="submit"
            disabled={loading}
            className="py-2 px-4 text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 disabled:opacity-50 rounded-md transition-colors"
          >
            {loading ? 'Saving…' : 'Save changes'}
          </button>
        </form>
      </div>

      <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-6 mt-6">
        <h2 className="text-sm font-medium text-gray-900 dark:text-gray-100 mb-4">Display</h2>
        <label className="flex items-center gap-3 cursor-pointer" onClick={() => setUse24h((v) => !v)}>
          <button
            type="button"
            role="switch"
            aria-checked={use24h}
            className={`relative inline-flex h-5 w-9 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors ${use24h ? 'bg-blue-600' : 'bg-gray-300 dark:bg-gray-600'}`}
          >
            <span className={`inline-block h-4 w-4 rounded-full bg-white shadow transition-transform ${use24h ? 'translate-x-4' : 'translate-x-0'}`} />
          </button>
          <span className="text-sm text-gray-700 dark:text-gray-300">Use 24-hour time</span>
        </label>
      </div>

      <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-6 mt-6">
        <h2 className="text-sm font-medium text-gray-900 dark:text-gray-100 mb-4">Logging</h2>
        <label className="block text-xs font-medium text-gray-600 dark:text-gray-400 mb-2">Log Level</label>
        <div className="flex gap-2">
          {logLevels.map((level) => (
            <button
              key={level}
              onClick={() => setLogLevel.mutate(level)}
              disabled={setLogLevel.isPending}
              className={`px-3 py-1.5 rounded text-sm font-medium capitalize transition-colors ${
                settings?.log_level === level
                  ? 'bg-blue-600 text-white'
                  : 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-600'
              }`}
            >
              {level}
            </button>
          ))}
        </div>
        {setLogLevel.isError && (
          <p className="mt-2 text-xs text-red-600 dark:text-red-400">{(setLogLevel.error as Error).message}</p>
        )}
      </div>
    </div>
  )
}
