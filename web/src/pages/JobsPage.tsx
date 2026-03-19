import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { api } from '../api/client'
import type { SyncJob, JobRequest } from '../api/types'
import { StatusBadge } from '../components/StatusBadge'

const empty: JobRequest = {
  name: '', connection_id: '', remote_path: '/', local_dest: '',
  interval_value: 60, interval_unit: 'minutes', concurrency: 1, enabled: true,
}

export function JobsPage() {
  const qc = useQueryClient()
  const [modal, setModal] = useState<{ open: boolean; editing: SyncJob | null }>({ open: false, editing: null })
  const [form, setForm] = useState<JobRequest>(empty)

  const { data: jobs = [], isLoading } = useQuery({ queryKey: ['jobs'], queryFn: api.jobs.list })
  const { data: connections = [] } = useQuery({ queryKey: ['connections'], queryFn: api.connections.list })

  const save = useMutation({
    mutationFn: (req: JobRequest) =>
      modal.editing ? api.jobs.update(modal.editing.id, req) : api.jobs.create(req),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['jobs'] }); closeModal() },
  })

  const remove = useMutation({
    mutationFn: api.jobs.delete,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['jobs'] }),
  })

  const trigger = useMutation({
    mutationFn: api.jobs.trigger,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['jobs'] }),
  })

  function openCreate() { setForm(empty); setModal({ open: true, editing: null }) }
  function openEdit(j: SyncJob) {
    setForm({
      name: j.name, connection_id: j.connection_id, remote_path: j.remote_path,
      local_dest: j.local_dest, interval_value: j.interval_value, interval_unit: j.interval_unit,
      concurrency: j.concurrency, enabled: j.enabled,
    })
    setModal({ open: true, editing: j })
  }
  function closeModal() { setModal({ open: false, editing: null }); save.reset() }

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-xl font-semibold text-gray-900">Sync Jobs</h1>
        <button onClick={openCreate} className="btn-primary">Add Job</button>
      </div>

      {isLoading && <p className="text-gray-500 text-sm">Loading…</p>}
      {!isLoading && jobs.length === 0 && (
        <p className="text-gray-400 text-sm">No jobs yet. Add one to get started.</p>
      )}

      <div className="space-y-2">
        {jobs.map((j) => (
          <div key={j.id} className="bg-white border border-gray-200 rounded-lg px-4 py-3 flex items-center gap-4">
            <div className="flex-1 min-w-0">
              <Link to={`/jobs/${j.id}`} className="font-medium text-gray-900 text-sm hover:text-blue-600">
                {j.name}
              </Link>
              <p className="text-xs text-gray-500">{j.remote_path} → {j.local_dest}</p>
              <p className="text-xs text-gray-400">
                Every {j.interval_value} {j.interval_unit} · {j.concurrency} concurrent · {j.enabled ? 'enabled' : 'disabled'}
              </p>
            </div>
            <StatusBadge status={j.enabled ? 'done' : 'skipped'} />
            <button
              onClick={() => trigger.mutate(j.id)}
              disabled={trigger.isPending}
              className="btn-secondary text-xs"
            >
              Run Now
            </button>
            <button onClick={() => openEdit(j)} className="btn-secondary text-xs">Edit</button>
            <button
              onClick={() => { if (confirm(`Delete "${j.name}"?`)) remove.mutate(j.id) }}
              className="btn-danger text-xs"
            >
              Delete
            </button>
          </div>
        ))}
      </div>

      {modal.open && (
        <div className="fixed inset-0 bg-black/40 flex items-center justify-center z-50">
          <div className="bg-white rounded-xl shadow-xl w-full max-w-lg mx-4 p-6 max-h-[90vh] overflow-y-auto">
            <div className="flex items-center justify-between mb-4">
              <h2 className="font-semibold text-gray-900">{modal.editing ? 'Edit Job' : 'Add Job'}</h2>
              <button onClick={closeModal} className="text-gray-400 hover:text-gray-600 text-xl leading-none">&times;</button>
            </div>
            <form onSubmit={(e) => { e.preventDefault(); save.mutate(form) }} className="space-y-3">
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
                <input className="input" value={form.remote_path} onChange={(e) => setForm({ ...form, remote_path: e.target.value })} required />
              </Field>
              <Field label="Local Destination">
                <input className="input" value={form.local_dest} onChange={(e) => setForm({ ...form, local_dest: e.target.value })} required />
              </Field>
              <div className="flex gap-2">
                <Field label="Every" className="w-24">
                  <input className="input" type="number" min={1} value={form.interval_value} onChange={(e) => setForm({ ...form, interval_value: Number(e.target.value) })} required />
                </Field>
                <Field label="Unit" className="flex-1">
                  <select className="input" value={form.interval_unit} onChange={(e) => setForm({ ...form, interval_unit: e.target.value as JobRequest['interval_unit'] })}>
                    <option value="minutes">Minutes</option>
                    <option value="hours">Hours</option>
                    <option value="days">Days</option>
                  </select>
                </Field>
                <Field label="Concurrency" className="w-28">
                  <input className="input" type="number" min={1} max={20} value={form.concurrency} onChange={(e) => setForm({ ...form, concurrency: Number(e.target.value) })} required />
                </Field>
              </div>
              <label className="flex items-center gap-2 text-sm text-gray-700 cursor-pointer">
                <input type="checkbox" checked={form.enabled} onChange={(e) => setForm({ ...form, enabled: e.target.checked })} />
                Enabled
              </label>
              {save.isError && <p className="text-red-600 text-sm">{(save.error as Error).message}</p>}
              <div className="flex justify-end gap-2 pt-2">
                <button type="button" onClick={closeModal} className="btn-secondary">Cancel</button>
                <button type="submit" disabled={save.isPending} className="btn-primary">
                  {save.isPending ? 'Saving…' : 'Save'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  )
}

function Field({ label, children, className }: { label: string; children: React.ReactNode; className?: string }) {
  return (
    <div className={className}>
      <label className="block text-xs font-medium text-gray-600 mb-1">{label}</label>
      {children}
    </div>
  )
}
