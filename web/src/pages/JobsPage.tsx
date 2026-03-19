import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { api } from '../api/client'
import type { SyncJob } from '../api/types'
import { StatusBadge } from '../components/StatusBadge'
import { JobFormModal } from '../components/JobFormModal'

export function JobsPage() {
  const qc = useQueryClient()
  const [modalOpen, setModalOpen] = useState(false)
  const [editingJob, setEditingJob] = useState<SyncJob | null>(null)

  const { data: jobs = [], isLoading } = useQuery({ queryKey: ['jobs'], queryFn: api.jobs.list })

  const remove = useMutation({
    mutationFn: api.jobs.delete,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['jobs'] }),
  })

  const trigger = useMutation({
    mutationFn: api.jobs.trigger,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['jobs'] }),
  })

  function openCreate() { setEditingJob(null); setModalOpen(true) }
  function openEdit(j: SyncJob) { setEditingJob(j); setModalOpen(true) }
  function closeModal() { setModalOpen(false); setEditingJob(null) }

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-xl font-semibold text-gray-900 dark:text-gray-100">Sync Jobs</h1>
        <button onClick={openCreate} className="btn-primary">Add Job</button>
      </div>

      {isLoading && <p className="text-gray-500 dark:text-gray-400 text-sm">Loading…</p>}
      {!isLoading && jobs.length === 0 && (
        <p className="text-gray-400 dark:text-gray-500 text-sm">No jobs yet. Add one to get started.</p>
      )}

      <div className="space-y-2">
        {jobs.map((j) => (
          <div key={j.id} className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg px-4 py-3 flex items-center gap-4">
            <div className="flex-1 min-w-0">
              <Link to={`/jobs/${j.id}`} className="font-medium text-gray-900 dark:text-gray-100 text-sm hover:text-blue-600 dark:hover:text-gray-200">
                {j.name}
              </Link>
              <p className="text-xs text-gray-500 dark:text-gray-400">{j.remote_path} → {j.local_dest}</p>
              <p className="text-xs text-gray-400 dark:text-gray-500">
                Every {j.interval_value} {j.interval_unit} · {j.concurrency} concurrent · autosync {j.enabled ? 'enabled' : 'disabled'}
              </p>
            </div>
            <StatusBadge status={j.enabled ? 'done' : 'skipped'} />
            <button onClick={() => openEdit(j)} className="btn-secondary text-xs">Edit</button>
            <button
              onClick={() => trigger.mutate(j.id)}
              disabled={trigger.isPending}
              className="btn-secondary text-xs"
            >
              Run Now
            </button>
            <button
              onClick={() => { if (confirm(`Delete "${j.name}"?`)) remove.mutate(j.id) }}
              className="btn-danger text-xs"
            >
              Delete
            </button>
          </div>
        ))}
      </div>

      {modalOpen && (
        <JobFormModal editing={editingJob} onClose={closeModal} />
      )}
    </div>
  )
}
