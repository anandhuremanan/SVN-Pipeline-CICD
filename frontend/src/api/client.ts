export interface Artifact {
  name: string;
  path: string;
}

export interface Project {
  name: string;
  path: string;
  type: string;
  root_dir?: string;
}

export interface BuildStepResult {
  name: string;
  command: string;
  duration: number; // nanoseconds
  successful: boolean;
  exit_code: number;
  error_message?: string;
  stdout?: string;
  stderr?: string;
}

export interface BuildResult {
  project: Project;
  status: 'PENDING' | 'RUNNING' | 'SUCCESS' | 'FAILED' | 'SKIPPED';
  duration: number;
  steps?: BuildStepResult[];
  artifacts?: Artifact[];
  stdout?: string;
  stderr?: string;
  error_message?: string;
}

export interface Job {
  id: string;
  repository: string;
  revision: string;
  workspace: string;
  workspace_retained: boolean;
  status: 'PENDING' | 'RUNNING' | 'SUCCESS' | 'FAILED' | 'INTERRUPTED' | 'CANCELLED';
  started_at: string;
  finished_at?: string;
  duration: number; // nanoseconds
  stage_timings?: Record<string, number>;
  deployment_outputs?: Record<string, string>;
  build_results?: BuildResult[];
  artifacts?: Artifact[];
  error_message?: string;
}

export interface LogEntry {
  time: string;
  job_id: string;
  project?: string;
  step?: string;
  level: string;
  message: string;
}

export interface DoctorResult {
  all_ok: boolean;
  tools: Record<string, boolean>;
  workspace_ok: boolean;
  storage_ok: boolean;
  credentials_ok: boolean;
}

export interface RunPipelinePayload {
  repository_url?: string;
  pipeline_file?: string;
  pipeline_content?: string;
  username?: string;
  password?: string;
  revision?: string;
  keep_workspace?: boolean;
}

const API_BASE = '/api';

export async function fetchBuildHistory(): Promise<Job[]> {
  const res = await fetch(`${API_BASE}/builds`);
  if (!res.ok) throw new Error(`Failed to fetch history: ${res.statusText}`);
  const data = await res.json();
  return Array.isArray(data) ? data : [];
}

export async function fetchJobDetails(jobId: string): Promise<Job> {
  const res = await fetch(`${API_BASE}/builds/${jobId}`);
  if (!res.ok) throw new Error(`Failed to fetch job ${jobId}: ${res.statusText}`);
  return res.json();
}

export async function fetchJobLogs(jobId: string): Promise<LogEntry[]> {
  const res = await fetch(`${API_BASE}/builds/${jobId}/logs`);
  if (!res.ok) throw new Error(`Failed to fetch logs for ${jobId}`);
  const data = await res.json();
  return Array.isArray(data) ? data : [];
}

export async function fetchRunningJobs(): Promise<Job[]> {
  const res = await fetch(`${API_BASE}/status`);
  if (!res.ok) throw new Error(`Failed to fetch status`);
  const data = await res.json();
  return Array.isArray(data) ? data : [];
}

export async function fetchDoctorDiagnostics(): Promise<DoctorResult> {
  const res = await fetch(`${API_BASE}/doctor`);
  if (!res.ok) throw new Error(`Failed to run doctor diagnostics`);
  return res.json();
}

export async function triggerBuild(payload: RunPipelinePayload): Promise<{ job_id: string; status: string }> {
  const res = await fetch(`${API_BASE}/builds`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
  if (!res.ok) {
    const errorText = await res.text();
    throw new Error(errorText || `Failed to trigger build: ${res.statusText}`);
  }
  return res.json();
}
