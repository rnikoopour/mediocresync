import { describe, it, expect } from 'vitest'
import { isTerminalStatus, resolveTransferStatus } from './runStatus'

describe('isTerminalStatus', () => {
  it.each([
    { status: 'completed',      expected: true  },
    { status: 'failed',         expected: true  },
    { status: 'partial',        expected: true  },
    { status: 'canceled',       expected: true  },
    { status: 'server_stopped', expected: true  },
    { status: 'nothing_to_sync', expected: true },
    { status: 'running',        expected: false },
    { status: 'canceling',      expected: false },
    { status: 'pending',        expected: false },
  ])('$status → $expected', ({ status, expected }) => {
    expect(isTerminalStatus(status)).toBe(expected)
  })
})

describe('resolveTransferStatus', () => {
  it.each([
    { status: 'pending',     runEnded: true,  expected: 'not_copied' },
    { status: 'pending',     runEnded: false, expected: 'pending'    },
    { status: 'done',        runEnded: true,  expected: 'done'       },
    { status: 'done',        runEnded: false, expected: 'done'       },
    { status: 'in_progress', runEnded: false, expected: 'in_progress' },
    { status: 'failed',      runEnded: true,  expected: 'failed'     },
    { status: 'skipped',     runEnded: true,  expected: 'skipped'    },
  ])('$status + runEnded=$runEnded → $expected', ({ status, runEnded, expected }) => {
    expect(resolveTransferStatus(status, runEnded)).toBe(expected)
  })
})
