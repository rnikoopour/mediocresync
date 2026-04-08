export const TERMINAL_RUN_STATUSES = [
  'completed',
  'failed',
  'partial',
  'canceled',
  'server_stopped',
  'nothing_to_sync',
] as const

export function isTerminalStatus(status: string): boolean {
  return TERMINAL_RUN_STATUSES.includes(status as typeof TERMINAL_RUN_STATUSES[number])
}

// A pending transfer on an ended run was never started — display as not_copied.
export function resolveTransferStatus(status: string, runEnded: boolean): string {
  return runEnded && status === 'pending' ? 'not_copied' : status
}
