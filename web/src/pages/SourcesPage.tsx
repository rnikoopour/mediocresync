import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../api/client'
import type { Source, SourceRequest } from '../api/types'
import { StatusBadge } from '../components/StatusBadge'
import { Modal } from '../components/Modal'

const emptyForm: SourceRequest = {
  name: '', type: 'ftpes', host: '', port: 21, username: '', password: '',
  auth_type: 'none', auth_credential: '', skip_tls_verify: false, enable_epsv: false,
}

export function SourcesPage() {
  const qc = useQueryClient()
  const [modal, setModal] = useState<{ open: boolean; editing: Source | null }>({ open: false, editing: null })
  const [form, setForm] = useState<SourceRequest>(emptyForm)
  const [activeTab, setActiveTab] = useState<'general' | 'advanced'>('general')
  const [testResult, setTestResult] = useState<{ id: string; ok: boolean; error?: string } | null>(null)
  const [modalTestResult, setModalTestResult] = useState<{ ok: boolean; error?: string } | null>(null)

  const { data: sources = [], isLoading } = useQuery({ queryKey: ['sources'], queryFn: api.sources.list })

  const save = useMutation({
    mutationFn: (req: SourceRequest) =>
      modal.editing
        ? api.sources.update(modal.editing.id, req)
        : api.sources.create(req),
    onSuccess: (src) => {
      qc.invalidateQueries({ queryKey: ['sources'] })
      if (src.type === 'ftpes') test.mutate(src.id)
      closeModal()
    },
  })

  const remove = useMutation({
    mutationFn: api.sources.delete,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['sources'] }),
  })

  const test = useMutation({
    mutationFn: api.sources.test,
    onSuccess: (res, id) => setTestResult({ id, ...res }),
  })

  const testDirect = useMutation({
    mutationFn: (args: { form: typeof form; fallbackId?: string }) =>
      api.sources.testDirect({ ...args.form, fallback_id: args.fallbackId }),
    onSuccess: (res) => setModalTestResult(res),
  })

  function openCreate() {
    setForm(emptyForm)
    setActiveTab('general')
    setTestResult(null)
    setModalTestResult(null)
    setModal({ open: true, editing: null })
  }
  function openEdit(s: Source) {
    setForm({
      name: s.name, type: s.type,
      host: s.host, port: s.port, username: s.username, password: '',
      auth_type: s.auth_type, auth_credential: '',
      skip_tls_verify: s.skip_tls_verify, enable_epsv: s.enable_epsv,
    })
    setActiveTab('general')
    setTestResult(null)
    setModalTestResult(null)
    setModal({ open: true, editing: s })
  }
  function closeModal() { setModal({ open: false, editing: null }); save.reset() }

  const isFtpes = form.type === 'ftpes'

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-xl font-semibold text-gray-900 dark:text-gray-100">Sources</h1>
        <button onClick={openCreate} className="btn-primary">Add Source</button>
      </div>

      {isLoading && <p className="text-gray-500 dark:text-gray-400 text-sm">Loading…</p>}

      {!isLoading && sources.length === 0 && (
        <p className="text-gray-400 dark:text-gray-500 text-sm">No sources yet. Add one to get started.</p>
      )}

      <div className="space-y-2">
        {sources.map((s) => (
          <div key={s.id} className="card px-4 py-3 flex flex-wrap items-start gap-2 sm:gap-4">
            <div className="flex-1 min-w-0">
              <div className="flex items-center gap-2">
                <p className="font-medium text-gray-900 dark:text-gray-100 text-sm">{s.name}</p>
                <span className="text-xs text-gray-400 dark:text-gray-500 uppercase font-mono">{s.type}</span>
              </div>
              {s.type === 'ftpes' && (
                <p className="text-xs text-gray-500 dark:text-gray-400 break-all">{s.username}@{s.host}:{s.port}</p>
              )}
              {s.type === 'git' && (
                <p className="text-xs text-gray-500 dark:text-gray-400">{s.auth_type === 'none' ? 'No auth' : s.auth_type}</p>
              )}
            </div>
            {testResult?.id === s.id && (
              <StatusBadge status={testResult.ok ? 'done' : 'failed'} />
            )}
            <button onClick={() => openEdit(s)} className="btn-secondary text-xs">Edit</button>
            {s.type === 'ftpes' && (
              <button
                onClick={() => test.mutate(s.id)}
                disabled={test.isPending}
                className="btn-secondary text-xs"
              >
                Test
              </button>
            )}
            <button
              onClick={() => { if (confirm(`Delete "${s.name}"?`)) remove.mutate(s.id) }}
              className="btn-danger text-xs"
            >
              Delete
            </button>
          </div>
        ))}
      </div>

      {modal.open && (
        <Modal>
            {/* Header */}
            <div className="flex items-center justify-between px-6 py-4 border-b border-gray-200 dark:border-gray-700 shrink-0">
              <div className="flex items-center gap-3">
                <h2 className="font-semibold text-gray-900 dark:text-gray-100">
                  {modal.editing ? 'Edit Source' : 'Add Source'}
                </h2>
                <select
                  className="input py-1 text-sm w-28"
                  value={form.type}
                  onChange={(e) => setForm({ ...form, type: e.target.value as SourceRequest['type'], host: '', port: e.target.value === 'ftpes' ? 21 : 0, username: '', password: '', auth_type: 'none', auth_credential: '' })}
                  disabled={!!modal.editing}
                >
                  <option value="ftpes">FTPES</option>
                  <option value="git">Git</option>
                </select>
              </div>
              <button onClick={closeModal} className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 text-xl leading-none">&times;</button>
            </div>

            <form onSubmit={(e) => { e.preventDefault(); save.mutate(form) }} className="flex flex-col sm:flex-row flex-1 min-h-0 overflow-hidden">
              {/* Mobile horizontal tabs */}
              <div className="sm:hidden flex shrink-0 border-b border-gray-200 dark:border-gray-700">
                {(['general', 'advanced'] as const).map((tab) => (
                  <button
                    key={tab}
                    type="button"
                    onClick={() => setActiveTab(tab)}
                    className={`flex-1 py-2 text-sm capitalize ${
                      activeTab === tab
                        ? 'border-b-2 border-blue-600 text-blue-700 dark:text-gray-100 font-medium'
                        : 'text-gray-600 dark:text-gray-400'
                    }`}
                  >
                    {tab}
                  </button>
                ))}
              </div>
              {/* Desktop sidebar tabs */}
              <nav className="hidden sm:block w-36 border-r border-gray-200 dark:border-gray-700 py-3 shrink-0">
                {(['general', 'advanced'] as const).map((tab) => (
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
                      <Field label="Name">
                        <input className="input" value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} required />
                      </Field>
                      {isFtpes && (
                        <>
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
                        </>
                      )}
                      {!isFtpes && (
                        <>
                          <Field label="Auth Type">
                            <select className="input" value={form.auth_type} onChange={(e) => setForm({ ...form, auth_type: e.target.value })}>
                              <option value="none">None (public repos)</option>
                              <option value="token">Token</option>
                              <option value="ssh_key">SSH Key</option>
                            </select>
                          </Field>
                          {form.auth_type === 'token' && (
                            <Field label={modal.editing ? 'Token (leave blank to keep current)' : 'Token'}>
                              <input className="input" type="password" value={form.auth_credential} onChange={(e) => setForm({ ...form, auth_credential: e.target.value })} required={!modal.editing} />
                            </Field>
                          )}
                          {form.auth_type === 'ssh_key' && (
                            <Field label={modal.editing ? 'SSH Private Key (leave blank to keep current)' : 'SSH Private Key'}>
                              <textarea className="input font-mono text-xs" rows={6} value={form.auth_credential} onChange={(e) => setForm({ ...form, auth_credential: e.target.value })} required={!modal.editing} placeholder="-----BEGIN OPENSSH PRIVATE KEY-----" />
                            </Field>
                          )}
                        </>
                      )}
                    </>
                  )}

                  {activeTab === 'advanced' && isFtpes && (
                    <>
                      <div>
                        <label className="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300 cursor-pointer">
                          <input type="checkbox" checked={form.enable_epsv} onChange={(e) => setForm({ ...form, enable_epsv: e.target.checked })} />
                          Enable EPSV
                        </label>
                        <p className="text-xs text-gray-400 dark:text-gray-500 mt-1 ml-5">Extended Passive mode; disable if you see login: EOF errors</p>
                      </div>
                      <div>
                        <label className="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300 cursor-pointer">
                          <input type="checkbox" checked={form.skip_tls_verify} onChange={(e) => setForm({ ...form, skip_tls_verify: e.target.checked })} />
                          Skip TLS certificate verification
                        </label>
                        <p className="text-xs text-gray-400 dark:text-gray-500 mt-1 ml-5">Insecure; use only for self-signed certs</p>
                      </div>
                    </>
                  )}
                  {activeTab === 'advanced' && !isFtpes && (
                    <p className="text-xs text-gray-400 dark:text-gray-500">No advanced options for Git sources.</p>
                  )}
                </div>

                {/* Footer */}
                <div className="px-6 py-4 border-t border-gray-200 dark:border-gray-700 shrink-0">
                  {save.isError && <p className="text-red-600 dark:text-red-400 text-sm mb-3">{(save.error as Error).message}</p>}
                  <div className="flex justify-end gap-2">
                    {isFtpes && (
                      <div className="flex items-center gap-2 mr-auto">
                        <button
                          type="button"
                          onClick={() => testDirect.mutate({ form, fallbackId: modal.editing?.id })}
                          disabled={testDirect.isPending}
                          className="btn-secondary"
                        >
                          {testDirect.isPending ? 'Testing…' : 'Test'}
                        </button>
                        {modalTestResult && (
                          modalTestResult.ok
                            ? <span className="text-green-500 text-lg leading-none">✓</span>
                            : <span className="text-red-500 text-lg leading-none" title={modalTestResult.error}>✗</span>
                        )}
                      </div>
                    )}
                    <button type="button" onClick={closeModal} className="btn-secondary">Cancel</button>
                    <button type="submit" disabled={save.isPending} className="btn-primary">
                      {save.isPending ? 'Saving…' : 'Save'}
                    </button>
                  </div>
                </div>
              </div>
            </form>
        </Modal>
      )}
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
