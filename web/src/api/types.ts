export interface Source {
  id: string
  name: string
  type: 'ftpes' | 'git'
  host: string
  port: number
  username: string
  auth_type: string
  skip_tls_verify: boolean
  enable_epsv: boolean
  created_at: string
  updated_at: string
}

export interface SourceRequest {
  name: string
  type: 'ftpes' | 'git'
  host: string
  port: number
  username: string
  password: string
  auth_type: string
  auth_credential: string
  skip_tls_verify: boolean
  enable_epsv: boolean
}

export interface GitRepo {
  id: string
  url: string
  branch: string
}

export interface GitRepoRequest {
  url: string
  branch: string
}

export interface SyncJob {
  id: string
  name: string
  source_id: string
  remote_path: string
  local_dest: string
  interval_value: number
  interval_unit: 'minutes' | 'hours' | 'days'
  concurrency: number
  retry_attempts: number
  retry_delay_seconds: number
  enabled: boolean
  include_path_filters: string[]
  include_name_filters: string[]
  exclude_path_filters: string[]
  exclude_name_filters: string[]
  run_retention_days: number
  git_repos: GitRepo[]
  created_at: string
  updated_at: string
}

export interface JobRequest {
  name: string
  source_id: string
  remote_path: string
  local_dest: string
  interval_value: number
  interval_unit: 'minutes' | 'hours' | 'days'
  concurrency: number
  retry_attempts: number
  retry_delay_seconds: number
  enabled: boolean
  include_path_filters: string[]
  include_name_filters: string[]
  exclude_path_filters: string[]
  exclude_name_filters: string[]
  run_retention_days: number
  git_repos: GitRepoRequest[]
}

export interface Run {
  id: string
  job_id: string
  status: 'running' | 'completed' | 'partial' | 'nothing_to_sync' | 'failed' | 'canceled' | 'server_stopped'
  started_at: string
  finished_at?: string
  total_files: number
  copied_files: number
  skipped_files: number
  failed_files: number
  total_size_bytes: number
  error_msg?: string
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
  previous_commit_hash?: string
  current_commit_hash?: string
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
  commit_hash?: string
  previous_commit_hash?: string
}

export interface PlanResult {
  files: PlanFile[]
  to_copy: number
  to_skip: number
}

export type LogLevel = 'debug' | 'info' | 'warn' | 'error'

export interface ServerSettings {
  log_level: LogLevel
}
