export interface Connection {
  id: string
  name: string
  host: string
  port: number
  username: string
  skip_tls_verify: boolean
  enable_epsv: boolean
  created_at: string
  updated_at: string
}

export interface ConnectionRequest {
  name: string
  host: string
  port: number
  username: string
  password: string
  skip_tls_verify: boolean
  enable_epsv: boolean
}

export interface SyncJob {
  id: string
  name: string
  connection_id: string
  remote_path: string
  local_dest: string
  interval_value: number
  interval_unit: 'minutes' | 'hours' | 'days'
  concurrency: number
  retry_attempts: number
  retry_delay_seconds: number
  enabled: boolean
  include_filters: string[]
  exclude_filters: string[]
  created_at: string
  updated_at: string
}

export interface JobRequest {
  name: string
  connection_id: string
  remote_path: string
  local_dest: string
  interval_value: number
  interval_unit: 'minutes' | 'hours' | 'days'
  concurrency: number
  retry_attempts: number
  retry_delay_seconds: number
  enabled: boolean
  include_filters: string[]
  exclude_filters: string[]
}

export interface Run {
  id: string
  job_id: string
  status: 'running' | 'completed' | 'failed' | 'canceled' | 'server_stopped'
  started_at: string
  finished_at?: string
  total_files: number
  copied_files: number
  skipped_files: number
  failed_files: number
  total_size_bytes: number
  transfers?: Transfer[]
}

export interface Transfer {
  id: string
  remote_path: string
  local_path: string
  size_bytes: number
  bytes_xferred: number
  duration_ms?: number
  status: 'pending' | 'in_progress' | 'done' | 'skipped' | 'failed'
  error_msg?: string
  started_at?: string
  finished_at?: string
}

export interface ProgressEvent {
  run_id: string
  transfer_id: string
  remote_path: string
  size_bytes: number
  bytes_xferred: number
  percent: number
  speed_bps: number
  status: Transfer['status']
  error?: string
}

export interface TestResult {
  ok: boolean
  error?: string
}

export interface BrowseEntry {
  name: string
  path: string
  is_dir: boolean
}

export interface PlanFile {
  remote_path: string
  local_path: string
  size_bytes: number
  mtime: string
  action: 'copy' | 'skip'
}

export interface PlanResult {
  files: PlanFile[]
  to_copy: number
  to_skip: number
}
