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
