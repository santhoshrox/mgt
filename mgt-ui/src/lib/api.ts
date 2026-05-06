// mgt-be REST client. All requests use cookie-based session auth. The base
// URL comes from VITE_API_URL (set in docker-compose / .env).

const BASE_URL = (import.meta.env.VITE_API_URL ?? 'http://localhost:8080').replace(/\/$/, '')

export class APIError extends Error {
  status: number
  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(`${BASE_URL}${path}`, {
    method,
    credentials: 'include',
    headers: body ? { 'Content-Type': 'application/json' } : undefined,
    body: body ? JSON.stringify(body) : undefined,
  })
  if (res.status === 401) throw new APIError(401, 'unauthenticated')
  if (!res.ok) {
    let msg = `${res.status} ${res.statusText}`
    try {
      const j = await res.json()
      if (j?.error) msg = j.error
    } catch { /* ignore */ }
    throw new APIError(res.status, msg)
  }
  if (res.status === 204) return undefined as T
  return (await res.json()) as T
}

// ── Types (mirror mgt-be wire formats) ────────────────────────────────────

export interface Me {
  id: number
  login: string
  avatar_url: string
}

export interface Repo {
  id: number
  owner: string
  name: string
  full_name: string
  default_branch: string
}

export interface StackBranch {
  id: number
  name: string
  parent: string
  position: number
}

export interface Stack {
  id: number
  trunk_branch: string
  branches: StackBranch[]
}

export interface PullRequest {
  number: number
  title: string
  body?: string
  state: string
  draft: boolean
  merged: boolean
  head_branch: string
  base_branch: string
  author_login: string
  author_avatar_url: string
  ci_state: string
  mergeable_state: string
  additions: number
  deletions: number
  comments: number
  html_url: string
  updated_at: string
}

export interface MergeQueueEntry {
  id: number
  pr_number: number
  state: string
  position: number
  attempts: number
  last_error?: string
  enqueued_at: string
}

// ── API methods ───────────────────────────────────────────────────────────

export const auth = {
  loginURL: () => `${BASE_URL}/auth/github/login`,
  me: () => request<Me>('GET', '/api/me'),
  logout: () => request<void>('POST', '/auth/logout'),
}

export const repos = {
  list: () => request<Repo[]>('GET', '/api/repos'),
  sync: () => request<Repo[]>('POST', '/api/repos/sync'),
}

export const stacks = {
  list: (repoId: number) => request<Stack[]>('GET', `/api/repos/${repoId}/stacks`),
  byBranch: (repoId: number, branch: string) =>
    request<Stack>('GET', `/api/repos/${repoId}/stacks/by-branch?name=${encodeURIComponent(branch)}`),
}

export const pulls = {
  list: (repoId: number, opts: { state?: string; author?: string; q?: string } = {}) => {
    const qs = new URLSearchParams()
    if (opts.state) qs.set('state', opts.state)
    if (opts.author) qs.set('author', opts.author)
    if (opts.q) qs.set('q', opts.q)
    const suffix = qs.toString() ? `?${qs}` : ''
    return request<PullRequest[]>('GET', `/api/repos/${repoId}/pulls${suffix}`)
  },
  get: (repoId: number, n: number) => request<PullRequest>('GET', `/api/repos/${repoId}/pulls/${n}`),
  describe: (repoId: number, n: number) =>
    request<{ body: string }>('POST', `/api/repos/${repoId}/pulls/${n}/describe`),
}

export const mergeQueue = {
  list: (repoId: number) => request<MergeQueueEntry[]>('GET', `/api/repos/${repoId}/merge-queue`),
  enqueue: (repoId: number, pr_number: number, method = 'squash') =>
    request<MergeQueueEntry>('POST', `/api/repos/${repoId}/merge-queue`, { pr_number, method }),
  cancel: (repoId: number, entryId: number) =>
    request<void>('DELETE', `/api/repos/${repoId}/merge-queue/${entryId}`),
}

// ── Helpers ───────────────────────────────────────────────────────────────

export function timeAgo(dateStr: string): string {
  const diff = Date.now() - new Date(dateStr).getTime()
  const mins = Math.floor(diff / 60_000)
  if (mins < 1) return 'just now'
  if (mins < 60) return `${mins}m ago`
  const hours = Math.floor(mins / 60)
  if (hours < 24) return `${hours}h ago`
  const days = Math.floor(hours / 24)
  if (days < 30) return `${days}d ago`
  return `${Math.floor(days / 30)}mo ago`
}
