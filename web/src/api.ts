export interface Exception {
  digest: string
  department: string
  because: string
  class: string
  options: string[]
  recommendation: string
  uncertainty?: string
  consequence: string
  deadline?: string
  requiredAuthority?: string
}

export interface Department {
  name: string
  procedure: string
  risk: string
  reversibility: string
}

export interface Job {
  id: number
  stepKey: string
  department: string
  because: string
  input: string
  risk: string
  reversibility: string
  confidence: string
  status: string
  attempts: number
  resultDigest?: string
  detail?: string
  createdAt: string
  claimedAt?: string
}

export interface RunDepartmentResult {
  status: 'queued' | 'already_queued' | 'escalated' | 'refused'
  jobId?: number
  reasons?: string[]
}

export interface Artifact {
  id: string
  kind: string
  predicateType: string
  subject: unknown
  predicate: Record<string, unknown>
}

export interface Health {
  status: string
  startedAt: string
}

export interface ArtifactHistoryPage {
  artifacts: Artifact[]
  nextBefore?: string
}

export const HISTORY_KINDS = ['goal', 'problem', 'hypothesis', 'spec', 'change', 'verdict', 'release', 'signal'] as const

export const BASE_URL = (import.meta.env.VITE_HR_SERVER_URL as string | undefined) ?? 'http://localhost:7777'

const TOKEN_STORAGE_KEY = 'hr-token'

export function getToken(): string {
  return localStorage.getItem(TOKEN_STORAGE_KEY) ?? ''
}

export function setToken(token: string): void {
  localStorage.setItem(TOKEN_STORAGE_KEY, token)
}

export function clearToken(): void {
  localStorage.removeItem(TOKEN_STORAGE_KEY)
}

export class UnauthorizedError extends Error {}

function authHeaders(): HeadersInit {
  const token = getToken()
  return token ? { Authorization: `Bearer ${token}` } : {}
}

async function checkResponse(res: Response, action: string): Promise<Response> {
  if (!res.ok) {
    const body = await res.text()
    if (res.status === 401) {
      throw new UnauthorizedError(body || 'missing or invalid token')
    }
    throw new Error(`${action}: ${res.status} ${body}`)
  }
  return res
}

export async function getHealth(): Promise<Health> {
  const res = await checkResponse(await fetch(`${BASE_URL}/healthz`), 'health check')
  return (await res.json()) as Health
}

export async function listExceptions(): Promise<Exception[]> {
  const res = await checkResponse(await fetch(`${BASE_URL}/v1/exceptions`, { headers: authHeaders() }), 'list exceptions')
  const data = (await res.json()) as { exceptions: Exception[] }
  return data.exceptions
}

export async function resolveException(digest: string, option: string): Promise<void> {
  await checkResponse(
    await fetch(`${BASE_URL}/v1/exceptions/${encodeURIComponent(digest)}/resolve`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', ...authHeaders() },
      body: JSON.stringify({ option }),
    }),
    'resolve exception',
  )
}

export async function listDepartments(): Promise<Department[]> {
  const res = await checkResponse(await fetch(`${BASE_URL}/v1/departments`, { headers: authHeaders() }), 'list departments')
  const data = (await res.json()) as { departments: Department[] }
  return data.departments
}

export async function runDepartment(name: string, input: string): Promise<RunDepartmentResult> {
  const res = await checkResponse(
    await fetch(`${BASE_URL}/v1/departments/${encodeURIComponent(name)}/run`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', ...authHeaders() },
      body: JSON.stringify({ input }),
    }),
    'run department',
  )
  return (await res.json()) as RunDepartmentResult
}

export async function listJobs(): Promise<Job[]> {
  const res = await checkResponse(await fetch(`${BASE_URL}/v1/jobs`, { headers: authHeaders() }), 'list jobs')
  const data = (await res.json()) as { jobs: Job[] }
  return data.jobs
}

export async function getArtifact(digest: string): Promise<Artifact> {
  const res = await checkResponse(
    await fetch(`${BASE_URL}/v1/artifacts/${encodeURIComponent(digest)}`, { headers: authHeaders() }),
    'get artifact',
  )
  return (await res.json()) as Artifact
}

// Browses the GOAL->PROBLEM->HYPOTHESIS->SPEC->CHANGE->VERDICT->RELEASE chain
// beyond listJobs's last-50 window. Exceptions are excluded server-side -
// they have their own scoped view via listExceptions.
export async function listArtifactHistory(opts: { before?: string; kind?: string } = {}): Promise<ArtifactHistoryPage> {
  const params = new URLSearchParams()
  if (opts.before) params.set('before', opts.before)
  if (opts.kind) params.set('kind', opts.kind)
  const qs = params.toString()
  const res = await checkResponse(
    await fetch(`${BASE_URL}/v1/history${qs ? `?${qs}` : ''}`, { headers: authHeaders() }),
    'list artifact history',
  )
  return (await res.json()) as ArtifactHistoryPage
}

// EventSource cannot set custom headers, so the token travels as a query
// parameter instead - the server accepts both (see internal/hrserver/auth.go).
export function subscribeToExceptionChanges(onChange: () => void): () => void {
  const es = new EventSource(`${BASE_URL}/v1/exceptions/stream?token=${encodeURIComponent(getToken())}`)
  es.addEventListener('changed', () => onChange())
  return () => es.close()
}

export function subscribeToJobChanges(onChange: () => void): () => void {
  const es = new EventSource(`${BASE_URL}/v1/jobs/stream?token=${encodeURIComponent(getToken())}`)
  es.addEventListener('changed', () => onChange())
  return () => es.close()
}
