import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api } from '../api/client'
import type { SyncJob, JobRequest } from '../api/types'
import { RemoteBrowser } from './RemoteBrowser'
import { LocalBrowser } from './LocalBrowser'
import { usePlan } from '../context/PlanContext'

interface Props {
  editing: SyncJob | null
  onClose: () => void
}

const empty: JobRequest = {
  name: '', connection_id: '', remote_path: '/', local_dest: '',
  interval_value: 60, interval_unit: 'minutes', concurrency: 1,
  retry_attempts: 3, retry_delay_seconds: 2, enabled: true,
  include_path_filters: [], include_name_filters: [],
  exclude_path_filters: [], exclude_name_filters: [],
}

function jobToForm(j: SyncJob): JobRequest {
  return {
    name: j.name, connection_id: j.connection_id, remote_path: j.remote_path,
    local_dest: j.local_dest, interval_value: j.interval_value, interval_unit: j.interval_unit,
    concurrency: j.concurrency, retry_attempts: j.retry_attempts, retry_delay_seconds: j.retry_delay_seconds,
    enabled: j.enabled,
    include_path_filters: j.include_path_filters ?? [],
    include_name_filters: j.include_name_filters ?? [],
    exclude_path_filters: j.exclude_path_filters ?? [],
    exclude_name_filters: j.exclude_name_filters ?? [],
  }
}

export function JobFormModal({ editing, onClose }: Props) {
  const qc = useQueryClient()
  const { dismissPlan } = usePlan()
  const [form, setForm] = useState<JobRequest>(editing ? jobToForm(editing) : empty)
  const [activeTab, setActiveTab] = useState<'general' | 'filters'>('general')
  const [browserOpen, setBrowserOpen] = useState(false)
  const [localBrowserOpen, setLocalBrowserOpen] = useState(false)

  const { data: connections = [] } = useQuery({ queryKey: ['connections'], queryFn: api.connections.list })

  const save = useMutation({
    mutationFn: (req: JobRequest) =>
      editing ? api.jobs.update(editing.id, req) : api.jobs.create(req),
    onSuccess: () => {
      if (editing) dismissPlan(editing.id)
      qc.invalidateQueries({ queryKey: ['jobs'] })
      onClose()
    },
  })

  return (
    <>
      <div className="fixed inset-0 bg-black/40 flex items-center justify-center z-50">
        <div className="bg-white dark:bg-gray-800 rounded-xl shadow-xl w-full max-w-2xl mx-4 h-[90vh] flex flex-col">
          {/* Header */}
          <div className="flex items-center justify-between px-6 py-4 border-b border-gray-200 dark:border-gray-700 shrink-0">
            <h2 className="font-semibold text-gray-900 dark:text-gray-100">{editing ? 'Edit Job' : 'Add Job'}</h2>
            <button onClick={onClose} className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 text-xl leading-none">&times;</button>
          </div>

          <form onSubmit={(e) => { e.preventDefault(); save.mutate(form) }} className="flex flex-1 min-h-0">
            {/* Sidebar tabs */}
            <nav className="w-36 border-r border-gray-200 dark:border-gray-700 py-3 shrink-0">
              {(['general', 'filters'] as const).map((tab) => (
                <button
                  key={tab}
                  type="button"
                  onClick={() => setActiveTab(tab)}
                  className={`w-full text-left px-4 py-2 text-sm capitalize ${
                    activeTab === tab
                      ? 'bg-blue-50 dark:bg-gray-700 text-blue-700 dark:text-gray-100 font-medium border-r-2 border-blue-600 dark:border-gray-400'
                      : 'text-gray-600 dark:text-gray-400 hover:bg-gray-50 dark:hover:bg-gray-700'
                  }`}
                >
                  {tab}
                </button>
              ))}
            </nav>

            {/* Tab content */}
            <div className="flex-1 flex flex-col min-h-0">
              <div className="flex-1 overflow-y-auto px-6 py-4 space-y-3">
                {activeTab === 'general' && (
                  <>
                    <p className="text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wide pt-1">General</p>
                    <Field label="Name">
                      <input className="input" value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} required />
                    </Field>
                    <Field label="Connection">
                      <select className="input" value={form.connection_id} onChange={(e) => setForm({ ...form, connection_id: e.target.value })} required>
                        <option value="">Select a connection…</option>
                        {connections.map((c) => <option key={c.id} value={c.id}>{c.name}</option>)}
                      </select>
                    </Field>
                    <Field label="Remote Path">
                      <div className="flex gap-2">
                        <input className="input flex-1" value={form.remote_path} onChange={(e) => setForm({ ...form, remote_path: e.target.value })} required />
                        <button
                          type="button"
                          onClick={() => setBrowserOpen(true)}
                          disabled={!form.connection_id}
                          className="btn-secondary text-xs shrink-0"
                          title={form.connection_id ? 'Browse remote server' : 'Select a connection first'}
                        >
                          Browse
                        </button>
                      </div>
                    </Field>
                    <Field label="Local Destination">
                      <div className="flex gap-2">
                        <input className="input flex-1" value={form.local_dest} onChange={(e) => setForm({ ...form, local_dest: e.target.value })} required />
                        <button
                          type="button"
                          onClick={() => setLocalBrowserOpen(true)}
                          className="btn-secondary text-xs shrink-0"
                        >
                          Browse
                        </button>
                      </div>
                    </Field>
                    <div className="flex gap-2">
                      <Field label="Max Concurrent Downloads" className="w-48">
                        <input className="input" type="number" min={1} max={20} value={form.concurrency} onChange={(e) => setForm({ ...form, concurrency: Number(e.target.value) })} required />
                      </Field>
                      <Field label="Max Retries" className="w-28">
                        <input className="input" type="number" min={1} value={form.retry_attempts} onChange={(e) => setForm({ ...form, retry_attempts: Number(e.target.value) })} required />
                      </Field>
                      <Field label="Backoff (seconds)" className="w-36">
                        <input className="input" type="number" min={0} value={form.retry_delay_seconds} onChange={(e) => setForm({ ...form, retry_delay_seconds: Number(e.target.value) })} required />
                      </Field>
                    </div>

                    <p className="text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wide pt-1">Autosync</p>
                    <div className="flex items-center gap-3">
                      <label className="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300 cursor-pointer shrink-0 w-28">
                        <span
                          role="switch"
                          aria-checked={form.enabled}
                          onClick={() => setForm({ ...form, enabled: !form.enabled })}
                          className={`relative inline-flex h-5 w-9 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors ${form.enabled ? 'bg-blue-600' : 'bg-gray-300 dark:bg-gray-600'}`}
                        >
                          <span className={`inline-block h-4 w-4 rounded-full bg-white shadow transition-transform ${form.enabled ? 'translate-x-4' : 'translate-x-0'}`} />
                        </span>
                        {form.enabled ? 'Enabled' : 'Disabled'}
                      </label>
                      <span className="text-sm text-gray-500 dark:text-gray-400 shrink-0">every</span>
                      <input className="input w-20" type="number" min={1} value={form.interval_value} onChange={(e) => setForm({ ...form, interval_value: Number(e.target.value) })} required />
                      <select className="input flex-1" value={form.interval_unit} onChange={(e) => setForm({ ...form, interval_unit: e.target.value as JobRequest['interval_unit'] })}>
                        <option value="minutes">Minutes</option>
                        <option value="hours">Hours</option>
                        <option value="days">Days</option>
                      </select>
                    </div>
                  </>
                )}

                {activeTab === 'filters' && (
                  <>
                    <p className="text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wide pt-1">Filters</p>
                    <p className="text-xs text-gray-400 dark:text-gray-500">
                      Path filters scope which directories are searched. Name filters scope which files within those directories are included. Both groups must match (AND logic).
                    </p>
                    <Field label="Include Path Filters">
                      <p className="text-xs text-gray-400 dark:text-gray-500 mb-1">Only descend into these subdirectories. Empty = search all directories.</p>
                      <FilterList
                        values={form.include_path_filters}
                        onChange={(v) => setForm({ ...form, include_path_filters: v })}
                        placeholder="e.g. alpha  or  projects/work"
                      />
                    </Field>
                    <Field label="Include Name Filters">
                      <p className="text-xs text-gray-400 dark:text-gray-500 mb-1">Only include files whose basename matches. Empty = include all filenames.</p>
                      <FilterList
                        values={form.include_name_filters}
                        onChange={(v) => setForm({ ...form, include_name_filters: v })}
                        placeholder="e.g. *.dat  or  *.bin"
                      />
                    </Field>
                    <Field label="Exclude Path Filters">
                      <p className="text-xs text-gray-400 dark:text-gray-500 mb-1">Skip files under these subdirectories.</p>
                      <FilterList
                        values={form.exclude_path_filters}
                        onChange={(v) => setForm({ ...form, exclude_path_filters: v })}
                        placeholder="e.g. tmp  or  misc"
                      />
                    </Field>
                    <Field label="Exclude Name Filters">
                      <p className="text-xs text-gray-400 dark:text-gray-500 mb-1">Skip files whose basename matches any of these patterns.</p>
                      <FilterList
                        values={form.exclude_name_filters}
                        onChange={(v) => setForm({ ...form, exclude_name_filters: v })}
                        placeholder="e.g. *.tmp  or  *.cfg"
                      />
                    </Field>
                  </>
                )}
              </div>

              {/* Footer */}
              <div className="px-6 py-4 border-t border-gray-200 dark:border-gray-700 shrink-0">
                {save.isError && <p className="text-red-600 dark:text-red-400 text-sm mb-3">{(save.error as Error).message}</p>}
                <div className="flex justify-end gap-2">
                  <button type="button" onClick={onClose} className="btn-secondary">Cancel</button>
                  <button type="submit" disabled={save.isPending} className="btn-primary">
                    {save.isPending ? 'Saving…' : 'Save'}
                  </button>
                </div>
              </div>
            </div>
          </form>
        </div>
      </div>

      {browserOpen && form.connection_id && (
        <RemoteBrowser
          connectionId={form.connection_id}
          initialPath={form.remote_path || '/'}
          onSelect={(path) => setForm({ ...form, remote_path: path })}
          onClose={() => setBrowserOpen(false)}
        />
      )}

      {localBrowserOpen && (
        <LocalBrowser
          initialPath={form.local_dest || '/'}
          onSelect={(path) => setForm({ ...form, local_dest: path })}
          onClose={() => setLocalBrowserOpen(false)}
        />
      )}
    </>
  )
}

function FilterList({ values, onChange, placeholder }: {
  values: string[]
  onChange: (v: string[]) => void
  placeholder: string
}) {
  const [draft, setDraft] = useState('')

  function add() {
    const trimmed = draft.trim()
    if (!trimmed || values.includes(trimmed)) return
    onChange([...values, trimmed])
    setDraft('')
  }

  return (
    <div className="space-y-1">
      {values.map((v, i) => (
        <div key={i} className="flex items-center gap-2 bg-gray-50 dark:bg-gray-700 rounded px-2 py-1">
          <span className="flex-1 font-mono text-xs text-gray-700 dark:text-gray-300">{v}</span>
          <button
            type="button"
            onClick={() => onChange(values.filter((_, idx) => idx !== i))}
            className="text-gray-400 hover:text-red-500 dark:hover:text-red-400 text-xs leading-none"
          >
            &times;
          </button>
        </div>
      ))}
      <div className="flex gap-2">
        <input
          className="input flex-1 text-xs font-mono"
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => { if (e.key === 'Enter') { e.preventDefault(); add() } }}
          placeholder={placeholder}
        />
        <button type="button" onClick={add} className="btn-secondary text-xs shrink-0">Add</button>
      </div>
    </div>
  )
}

function Field({ label, children, className }: { label: string; children: React.ReactNode; className?: string }) {
  return (
    <div className={className}>
      <label className="block text-xs font-medium text-gray-600 dark:text-gray-400 mb-1">{label}</label>
      {children}
    </div>
  )
}
