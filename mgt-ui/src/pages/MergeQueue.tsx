import { useCallback, useEffect, useState } from 'react'
import { ListOrdered, Plus, X, AlertCircle, RefreshCw } from 'lucide-react'
import { mergeQueue, pulls, timeAgo, type MergeQueueEntry, type PullRequest } from '../lib/api'
import { Loading, Empty } from '../components/PageState'
import RepoPicker from '../components/RepoPicker'
import { useRepoSelector } from '../lib/useRepo'

const stateColor: Record<string, string> = {
  queued: 'bg-gray-700 text-gray-300',
  integrating: 'bg-amber-500/20 text-amber-300',
  awaiting_ci: 'bg-amber-500/20 text-amber-300',
  merging: 'bg-cyan-500/20 text-cyan-300',
  merged: 'bg-emerald-500/20 text-emerald-300',
  failed: 'bg-rose-500/20 text-rose-300',
  cancelled: 'bg-gray-800 text-gray-500',
}

export default function MergeQueue() {
  const repo = useRepoSelector()
  const [entries, setEntries] = useState<MergeQueueEntry[]>([])
  const [openPRs, setOpenPRs] = useState<PullRequest[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [picker, setPicker] = useState(false)

  const refresh = useCallback(async () => {
    if (!repo.active) return
    setLoading(true)
    try {
      setError(null)
      const [q, prs] = await Promise.all([
        mergeQueue.list(repo.active.id),
        pulls.list(repo.active.id, { state: 'open' }),
      ])
      setEntries(q)
      setOpenPRs(prs)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'load failed')
    } finally {
      setLoading(false)
    }
  }, [repo.active])

  useEffect(() => { refresh() }, [refresh])

  // Light polling while a non-terminal entry exists.
  useEffect(() => {
    if (!repo.active) return
    const interesting = entries.some(e => ['queued', 'integrating', 'awaiting_ci', 'merging'].includes(e.state))
    if (!interesting) return
    const t = setInterval(refresh, 5000)
    return () => clearInterval(t)
  }, [entries, repo.active, refresh])

  async function enqueue(prNumber: number) {
    if (!repo.active) return
    try {
      await mergeQueue.enqueue(repo.active.id, prNumber)
      setPicker(false)
      refresh()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'enqueue failed')
    }
  }

  async function cancel(id: number) {
    if (!repo.active) return
    try {
      await mergeQueue.cancel(repo.active.id, id)
      refresh()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'cancel failed')
    }
  }

  const queued = openPRs.filter(p => !entries.some(e => e.pr_number === p.number))

  return (
    <div className="mx-auto max-w-5xl space-y-6 p-6 lg:p-8">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">Merge Queue</h1>
          <p className="mt-2 text-gray-400">FIFO merge queue managed by mgt-be.</p>
        </div>
        <div className="flex items-center gap-2">
          <RepoPicker repos={repo.repos} active={repo.active} onSelect={repo.select} onSync={repo.sync} syncing={repo.loading} />
          <button onClick={refresh} className="rounded-lg border border-gray-800 bg-gray-900 p-1.5 text-gray-400 hover:text-gray-200" title="Refresh">
            <RefreshCw className={`h-4 w-4 ${loading ? 'animate-spin' : ''}`} />
          </button>
          <button onClick={() => setPicker(true)} className="inline-flex items-center gap-1.5 rounded-lg bg-brand-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-brand-500">
            <Plus className="h-4 w-4" /> Enqueue PR
          </button>
        </div>
      </div>

      {error && (
        <div className="flex items-start gap-2 rounded-lg border border-rose-500/30 bg-rose-500/10 px-3 py-2 text-sm text-rose-300">
          <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" /> {error}
        </div>
      )}

      {!repo.active ? (
        <Empty icon={<ListOrdered className="h-10 w-10" />} message="Pick a repo to manage its queue." />
      ) : loading && entries.length === 0 ? (
        <Loading />
      ) : entries.length === 0 ? (
        <Empty icon={<ListOrdered className="h-10 w-10" />} message="Queue is empty. Click 'Enqueue PR' to add one." />
      ) : (
        <div className="space-y-2">
          {entries.map(entry => (
            <div key={entry.id} className="flex items-center gap-3 rounded-xl border border-gray-800 bg-gray-900/50 p-4">
              <div className="flex h-7 w-7 items-center justify-center rounded-full bg-gray-800 text-xs font-semibold text-gray-300">
                {entry.position}
              </div>
              <div className="min-w-0 flex-1">
                <p className="text-sm font-medium text-gray-200">PR #{entry.pr_number}</p>
                <p className="mt-0.5 text-xs text-gray-500">enqueued {timeAgo(entry.enqueued_at)} · attempts {entry.attempts}</p>
                {entry.last_error && <p className="mt-1 text-xs text-rose-400">{entry.last_error}</p>}
              </div>
              <span className={`rounded-full px-2.5 py-0.5 text-xs font-medium ${stateColor[entry.state] ?? 'bg-gray-800 text-gray-300'}`}>
                {entry.state}
              </span>
              {!['merged', 'cancelled', 'failed'].includes(entry.state) && (
                <button onClick={() => cancel(entry.id)} className="rounded-md p-1.5 text-gray-500 hover:bg-gray-800 hover:text-rose-400" title="Cancel">
                  <X className="h-4 w-4" />
                </button>
              )}
            </div>
          ))}
        </div>
      )}

      {picker && repo.active && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4" onClick={() => setPicker(false)}>
          <div className="max-h-[80vh] w-full max-w-lg overflow-hidden rounded-xl border border-gray-800 bg-gray-950" onClick={e => e.stopPropagation()}>
            <div className="flex items-center justify-between border-b border-gray-800 px-5 py-3">
              <h2 className="text-sm font-semibold">Pick a PR to enqueue</h2>
              <button onClick={() => setPicker(false)} className="text-gray-500 hover:text-gray-200"><X className="h-4 w-4" /></button>
            </div>
            <div className="max-h-[60vh] overflow-y-auto p-2">
              {queued.length === 0 ? (
                <p className="p-4 text-sm text-gray-500">All open PRs are already queued.</p>
              ) : queued.map(pr => (
                <button key={pr.number} onClick={() => enqueue(pr.number)} className="flex w-full items-center gap-3 rounded-lg px-3 py-2 text-left hover:bg-gray-900">
                  <span className="text-xs text-gray-500">#{pr.number}</span>
                  <span className="flex-1 truncate text-sm text-gray-200">{pr.title}</span>
                  <span className="text-xs text-gray-500">{pr.head_branch}</span>
                </button>
              ))}
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
