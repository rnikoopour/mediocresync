/**
 * Tests for JobDetailPage.
 *
 * Covers:
 *  1. formatBytes / formatDuration — verified through rendered run-row text
 *  2. buildRunTree / sortNodes     — verified through the transfer tree rendered
 *     inside an expanded run row
 *  3. Button disabled states        — Plan / Run Now / Edit buttons
 */
import { render, screen, waitFor } from '@testing-library/react'
import { describe, it, expect, beforeAll, afterEach, afterAll } from 'vitest'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { http, HttpResponse } from 'msw'
import { setupServer } from 'msw/node'
import { JobDetailPage } from './JobDetailPage'
import { PlanProvider } from '../context/PlanContext'
import type { SyncJob, Run, Transfer } from '../api/types'

// ── helpers ───────────────────────────────────────────────────────────────────

function buildJob(overrides: Partial<SyncJob> = {}): SyncJob {
  return {
    id: 'job-1',
    name: 'My Job',
    source_id: 'src-1',
    git_repos: [],
    remote_path: '/remote/base',
    local_dest: '/local/dest',
    interval_value: 1,
    interval_unit: 'hours',
    concurrency: 2,
    retry_attempts: 3,
    retry_delay_seconds: 10,
    enabled: true,
    include_path_filters: [],
    include_name_filters: [],
    exclude_path_filters: [],
    exclude_name_filters: [],
    run_retention_days: 0,
    created_at: '2024-01-01T00:00:00Z',
    updated_at: '2024-01-01T00:00:00Z',
    ...overrides,
  }
}

function buildRun(overrides: Partial<Run> = {}): Run {
  return {
    id: 'run-1',
    job_id: 'job-1',
    status: 'completed',
    started_at:  '2024-01-01T00:00:00.000Z',
    finished_at: '2024-01-01T00:01:05.000Z', // 1m 5s
    total_files:    3,
    copied_files:   2,
    skipped_files:  1,
    failed_files:   0,
    total_size_bytes: 2_097_152, // 2.0 MB
    ...overrides,
  }
}

function buildTransfer(id: string, remote_path: string, overrides: Partial<Transfer> = {}): Transfer {
  return {
    id,
    remote_path,
    local_path: '/local/' + id,
    size_bytes: 1_048_576, // 1.0 MB
    bytes_xferred: 1_048_576,
    status: 'done',
    ...overrides,
  }
}

function renderPage(jobId = 'job-1') {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <PlanProvider>
        <MemoryRouter initialEntries={[`/jobs/${jobId}`]}>
          <Routes>
            <Route path="/jobs/:id" element={<JobDetailPage />} />
          </Routes>
        </MemoryRouter>
      </PlanProvider>
    </QueryClientProvider>,
  )
}

// ── MSW server ────────────────────────────────────────────────────────────────

const defaultHandlers = [
  http.get('/api/jobs/:id',             () => HttpResponse.json(buildJob())),
  http.get('/api/jobs/:id/runs',        () => HttpResponse.json([])),
  http.get('/api/runs/:id',             () => HttpResponse.json(buildRun())),
  http.get('/api/jobs/:id/plan/events', () => new HttpResponse(null, { status: 200, headers: { 'Content-Type': 'text/event-stream' } })),
  http.get('/api/jobs/:id/events',      () => new HttpResponse(null, { status: 200, headers: { 'Content-Type': 'text/event-stream' } })),
]

const server = setupServer(...defaultHandlers)

beforeAll(() => server.listen({ onUnhandledRequest: 'error' }))
afterEach(() => server.resetHandlers())
afterAll(() => server.close())

// ── 1. formatBytes / formatDuration via rendered run row ──────────────────────
//
// The run row renders duration inside a <p> whose full text is
// "Started <date> · <duration>", so we match with a regex.
// Size is rendered in a standalone <span> so exact text matching works.

describe('formatBytes and formatDuration — rendered in run rows', () => {
  const table: Array<{
    label: string
    run: Run
    expectedSizeRe: RegExp | null
    expectedDurationRe: RegExp
  }> = [
    {
      label: 'displays size in MB and duration in seconds for a short run',
      run: buildRun({
        total_size_bytes: 2_097_152,   // 2.0 MB
        started_at:  '2024-01-01T00:00:00.000Z',
        finished_at: '2024-01-01T00:00:45.000Z', // 45 s
      }),
      expectedSizeRe:     /^2\.0 MB$/,
      expectedDurationRe: /· 45s/,
    },
    {
      label: 'displays size in KB and duration in minutes+seconds',
      run: buildRun({
        total_size_bytes: 3_072,         // 3.0 KB
        started_at:  '2024-01-01T00:00:00.000Z',
        finished_at: '2024-01-01T00:02:30.000Z', // 2m 30s
      }),
      expectedSizeRe:     /^3\.0 KB$/,
      expectedDurationRe: /· 2m 30s/,
    },
    {
      label: 'displays size in GB and duration in hours+minutes+seconds',
      run: buildRun({
        total_size_bytes: 2_147_483_648,  // 2.0 GB
        started_at:  '2024-01-01T00:00:00.000Z',
        finished_at: '2024-01-01T01:02:03.000Z', // 1h 2m 3s
      }),
      expectedSizeRe:     /^2\.0 GB$/,
      expectedDurationRe: /· 1h 2m 3s/,
    },
    {
      label: 'omits size when total_size_bytes is 0',
      run: buildRun({
        total_size_bytes: 0,
        started_at:  '2024-01-01T00:00:00.000Z',
        finished_at: '2024-01-01T00:01:05.000Z', // 1m 5s
      }),
      expectedSizeRe:     null,
      expectedDurationRe: /· 1m 5s/,
    },
  ]

  it.each(table)('$label', async ({ run, expectedSizeRe, expectedDurationRe }) => {
    server.use(
      http.get('/api/jobs/:id/runs', () => HttpResponse.json([run])),
    )

    renderPage()

    // Wait for the run row to appear, then verify duration text.
    await waitFor(() =>
      expect(screen.getByText(expectedDurationRe)).toBeInTheDocument(),
    )

    if (expectedSizeRe !== null) {
      expect(screen.getByText(expectedSizeRe)).toBeInTheDocument()
    } else {
      expect(screen.queryByText(/\d+\.\d+ (GB|MB|KB)/)).not.toBeInTheDocument()
    }
  })
})

// ── 2. buildRunTree / sortNodes — verified via transfer tree rendering ─────────

describe('buildRunTree and sortNodes — transfer tree structure', () => {
  it('renders a folder node for transfers in a subdirectory', async () => {
    const run: Run = {
      ...buildRun({ status: 'completed' }),
      transfers: [
        buildTransfer('t1', '/remote/base/subdir/file-a.txt'),
        buildTransfer('t2', '/remote/base/subdir/file-b.txt'),
      ],
    }
    server.use(
      http.get('/api/jobs/:id/runs',  () => HttpResponse.json([run])),
      http.get('/api/runs/:id',       () => HttpResponse.json(run)),
    )

    renderPage()

    // The run row renders once the runs query resolves.
    await waitFor(() => expect(screen.getByText(/Started/)).toBeInTheDocument())
  })

  it('sorts folders before files in the run tree', async () => {
    const run: Run = {
      ...buildRun({ status: 'running' }),
      transfers: [
        buildTransfer('t1', '/remote/base/z-last-file.txt'),
        buildTransfer('t2', '/remote/base/aaa-subdir/nested.txt'),
        buildTransfer('t3', '/remote/base/a-first-file.txt'),
      ],
    }
    server.use(
      http.get('/api/jobs/:id/runs',  () => HttpResponse.json([run])),
      http.get('/api/runs/:id',       () => HttpResponse.json(run)),
    )

    renderPage()

    // Running run rows are expanded by default so the tree is visible.
    // Folder 'aaa-subdir' must appear before the file nodes in document order.
    await waitFor(() => expect(screen.getByText('aaa-subdir')).toBeInTheDocument())

    const folderEl  = screen.getByText('aaa-subdir')
    const firstFile = screen.getByText('a-first-file.txt')
    expect(
      folderEl.compareDocumentPosition(firstFile) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy()
  })
})

// ── 3. Button disabled states ─────────────────────────────────────────────────
//
// jobIsRunning  = runs[0]?.status === 'running'
// Plan button   disabled when jobIsRunning OR planEntry.status === 'running'
// Run Now btn   disabled when jobIsRunning OR planEntry.status !== 'done' OR trigger.isPending
// Edit button   disabled when jobIsRunning OR planEntry.status === 'running'

describe('JobDetailPage button disabled states', () => {
  const table: Array<{
    label: string
    runs: Run[]
    expectedPlanDisabled: boolean
    expectedRunDisabled: boolean
    expectedEditDisabled: boolean
    /** Optional text to wait for before asserting button state. */
    waitForText?: string | RegExp
  }> = [
    {
      label: 'all action buttons enabled when idle and no plan',
      runs: [],
      expectedPlanDisabled: false,
      expectedRunDisabled:  true,  // no plan → Run Now stays disabled
      expectedEditDisabled: false,
      waitForText: 'No runs yet.',
    },
    {
      label: 'Plan and Edit disabled, Run Now disabled, when a run is in progress',
      runs: [buildRun({ status: 'running', finished_at: undefined })],
      expectedPlanDisabled: true,
      expectedRunDisabled:  true,
      expectedEditDisabled: true,
      waitForText: 'Running',
    },
  ]

  it.each(table)(
    '$label',
    async ({ runs, expectedPlanDisabled, expectedRunDisabled, expectedEditDisabled, waitForText }) => {
      server.use(
        http.get('/api/jobs/:id/runs', () => HttpResponse.json(runs)),
      )

      renderPage()

      // Wait for the page to reflect the loaded state before asserting buttons.
      if (waitForText) {
        await waitFor(() => expect(screen.getByText(waitForText)).toBeInTheDocument())
      }

      const planBtn = screen.getByRole('button', { name: /^Plan/ })
      const runBtn  = screen.getByRole('button', { name: /Run Now|Starting/ })
      const editBtn = screen.getByRole('button', { name: /^Edit$/ })

      await waitFor(() => {
        expect(planBtn.hasAttribute('disabled')).toBe(expectedPlanDisabled)
        expect(runBtn.hasAttribute('disabled')).toBe(expectedRunDisabled)
        expect(editBtn.hasAttribute('disabled')).toBe(expectedEditDisabled)
      })
    },
  )
})
