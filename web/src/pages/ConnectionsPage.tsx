import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../api/client'
import type { Connection, ConnectionRequest } from '../api/types'
import { StatusBadge } from '../components/StatusBadge'

const empty: ConnectionRequest = { name: '', host: '', port: 21, username: '', password: '', skip_tls_verify: false }

export function ConnectionsPage() {
  const qc = useQueryClient()
  const [modal, setModal] = useState<{ open: boolean; editing: Connection | null }>({ open: false, editing: null })
  const [form, setForm] = useState<ConnectionRequest>(empty)
  const [testResult, setTestResult] = useState<{ id: string; ok: boolean; error?: string } | null>(null)

  const { data: connections = [], isLoading } = useQuery({ queryKey: ['connections'], queryFn: api.connections.list })

  const save = useMutation({
    mutationFn: (req: ConnectionRequest) =>
      modal.editing
        ? api.connections.update(modal.editing.id, req)
        : api.connections.create(req),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['connections'] }); closeModal() },
  })

  const remove = useMutation({
    mutationFn: api.connections.delete,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['connections'] }),
  })

  const test = useMutation({
    mutationFn: api.connections.test,
    onSuccess: (res, id) => setTestResult({ id, ...res }),
  })

  function openCreate() { setForm(empty); setModal({ open: true, editing: null }) }
  function openEdit(c: Connection) {
    setForm({ name: c.name, host: c.host, port: c.port, username: c.username, password: '', skip_tls_verify: c.skip_tls_verify })
    setModal({ open: true, editing: c })
  }
  function closeModal() { setModal({ open: false, editing: null }); save.reset() }

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-xl font-semibold text-gray-900 dark:text-gray-100">Connections</h1>
        <button onClick={openCreate} className="btn-primary">Add Connection</button>
      </div>

      {isLoading && <p className="text-gray-500 dark:text-gray-400 text-sm">Loading…</p>}

      {!isLoading && connections.length === 0 && (
        <p className="text-gray-400 dark:text-gray-500 text-sm">No connections yet. Add one to get started.</p>
      )}

      <div className="space-y-2">
        {connections.map((c) => (
          <div key={c.id} className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg px-4 py-3 flex items-center gap-4">
            <div className="flex-1 min-w-0">
              <p className="font-medium text-gray-900 dark:text-gray-100 text-sm">{c.name}</p>
              <p className="text-xs text-gray-500 dark:text-gray-400">{c.username}@{c.host}:{c.port}</p>
            </div>
            {testResult?.id === c.id && (
              <StatusBadge status={testResult.ok ? 'done' : 'failed'} />
            )}
            <button
              onClick={() => test.mutate(c.id)}
              disabled={test.isPending}
              className="btn-secondary text-xs"
            >
              Test
            </button>
            <button onClick={() => openEdit(c)} className="btn-secondary text-xs">Edit</button>
            <button
              onClick={() => { if (confirm(`Delete "${c.name}"?`)) remove.mutate(c.id) }}
              className="btn-danger text-xs"
            >
              Delete
            </button>
          </div>
        ))}
      </div>

      {modal.open && (
        <Modal title={modal.editing ? 'Edit Connection' : 'Add Connection'} onClose={closeModal}>
          <form
            onSubmit={(e) => { e.preventDefault(); save.mutate(form) }}
            className="space-y-3"
          >
            <Field label="Name">
              <input className="input" value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} required />
            </Field>
            <div className="flex gap-2">
              <Field label="Host" className="flex-1">
                <input className="input" value={form.host} onChange={(e) => setForm({ ...form, host: e.target.value })} required />
              </Field>
              <Field label="Port" className="w-24">
                <input className="input" type="number" value={form.port} onChange={(e) => setForm({ ...form, port: Number(e.target.value) })} required />
              </Field>
            </div>
            <Field label="Username">
              <input className="input" value={form.username} onChange={(e) => setForm({ ...form, username: e.target.value })} required />
            </Field>
            <Field label={modal.editing ? 'Password (leave blank to keep current)' : 'Password'}>
              <input className="input" type="password" value={form.password} onChange={(e) => setForm({ ...form, password: e.target.value })} required={!modal.editing} />
            </Field>
            <label className="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300 cursor-pointer">
              <input type="checkbox" checked={form.skip_tls_verify} onChange={(e) => setForm({ ...form, skip_tls_verify: e.target.checked })} />
              Skip TLS certificate verification
            </label>
            {save.isError && <p className="text-red-600 dark:text-red-400 text-sm">{(save.error as Error).message}</p>}
            <div className="flex justify-end gap-2 pt-2">
              <button type="button" onClick={closeModal} className="btn-secondary">Cancel</button>
              <button type="submit" disabled={save.isPending} className="btn-primary">
                {save.isPending ? 'Saving…' : 'Save'}
              </button>
            </div>
          </form>
        </Modal>
      )}
    </div>
  )
}

function Modal({ title, onClose, children }: { title: string; onClose: () => void; children: React.ReactNode }) {
  return (
    <div className="fixed inset-0 bg-black/40 flex items-center justify-center z-50">
      <div className="bg-white dark:bg-gray-800 rounded-xl shadow-xl w-full max-w-md mx-4 p-6">
        <div className="flex items-center justify-between mb-4">
          <h2 className="font-semibold text-gray-900 dark:text-gray-100">{title}</h2>
          <button onClick={onClose} className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 text-xl leading-none">&times;</button>
        </div>
        {children}
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
