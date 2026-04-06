import { render, screen } from '@testing-library/react'
import { describe, it, expect } from 'vitest'
import { StatusBadge } from './StatusBadge'

describe('StatusBadge', () => {
  const statusLabelTable: Array<{ status: string; expectedLabel: string }> = [
    { status: 'running',        expectedLabel: 'Running'         },
    { status: 'in_progress',    expectedLabel: 'In Progress'     },
    { status: 'retrying',       expectedLabel: 'Retrying'        },
    { status: 'planning',       expectedLabel: 'Planning'        },
    { status: 'completed',      expectedLabel: 'Completed'       },
    { status: 'nothing_to_sync',expectedLabel: 'Nothing To Sync' },
    { status: 'done',           expectedLabel: 'Copied'          },
    { status: 'failed',         expectedLabel: 'Failed'          },
    { status: 'canceled',       expectedLabel: 'Canceled'        },
    { status: 'server_stopped', expectedLabel: 'Server Stopped'  },
    { status: 'pending',        expectedLabel: 'Pending'         },
    { status: 'skipped',        expectedLabel: 'Skipped'         },
    { status: 'not_copied',     expectedLabel: 'Not Copied'      },
  ]

  it.each(statusLabelTable)(
    'renders "$expectedLabel" for status "$status"',
    ({ status, expectedLabel }) => {
      render(<StatusBadge status={status} />)
      expect(screen.getByText(expectedLabel)).toBeInTheDocument()
    },
  )

  it('falls back to the raw status string for an unknown status value', () => {
    render(<StatusBadge status="totally_unknown" />)
    expect(screen.getByText('totally_unknown')).toBeInTheDocument()
  })
})
