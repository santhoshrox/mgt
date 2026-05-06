import { useEffect, useState } from 'react'
import { Bot, Sparkles, ExternalLink } from 'lucide-react'
import { pulls, type PullRequest } from '../lib/api'
import { Loading, ErrorState, Empty } from '../components/PageState'
import RepoPicker from '../components/RepoPicker'
import { useRepoSelector } from '../lib/useRepo'

export default function AIReviews() {
  const repo = useRepoSelector()
  const [items, setItems] = useState<PullRequest[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState<number | null>(null)
  const [output, setOutput] = useState<{ number: number; body: string } | null>(null)

  useEffect(() => {
    if (!repo.active) return
    setLoading(true)
    pulls.list(repo.active.id, { state: 'open' })
      .then(setItems)
      .catch(err => setError(err instanceof Error ? err.message : 'load failed'))
      .finally(() => setLoading(false))
  }, [repo.active])

  async function describe(n: number) {
    if (!repo.active) return
    setBusy(n)
    setError(null)
    setOutput(null)
    try {
      const res = await pulls.describe(repo.active.id, n)
      setOutput({ number: n, body: res.body })
    } catch (err) {
      setError(err instanceof Error ? err.message : 'describe failed')
    } finally {
      setBusy(null)
    }
  }

  return (
    <div className="mx-auto max-w-5xl space-y-6 p-6 lg:p-8">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">AI Reviews</h1>
          <p className="mt-2 text-gray-400">Have the server-configured LLM regenerate any open PR's description.</p>
        </div>
        <RepoPicker repos={repo.repos} active={repo.active} onSelect={repo.select} onSync={repo.sync} syncing={repo.loading} />
      </div>

      {error && <ErrorState message={error} />}
      {!repo.active ? (
        <Empty icon={<Bot className="h-10 w-10" />} message="Pick a repo." />
      ) : loading ? (
        <Loading />
      ) : items.length === 0 ? (
        <Empty icon={<Bot className="h-10 w-10" />} message="No open PRs to describe." />
      ) : (
        <div className="space-y-2">
          {items.map(pr => (
            <div key={pr.number} className="flex items-center gap-3 rounded-xl border border-gray-800 bg-gray-900/50 p-4">
              <div className="min-w-0 flex-1">
                <p className="truncate text-sm font-medium text-gray-200">{pr.title}</p>
                <p className="mt-0.5 text-xs text-gray-500">#{pr.number} · {pr.author_login} · {pr.head_branch}</p>
              </div>
              <a href={pr.html_url} target="_blank" rel="noopener noreferrer" className="rounded-md p-1.5 text-gray-500 hover:bg-gray-800 hover:text-gray-200" title="Open on GitHub">
                <ExternalLink className="h-4 w-4" />
              </a>
              <button onClick={() => describe(pr.number)} disabled={busy === pr.number}
                className="inline-flex items-center gap-1.5 rounded-lg bg-amber-500/15 px-3 py-1.5 text-sm font-medium text-amber-300 hover:bg-amber-500/25 disabled:opacity-50">
                <Sparkles className={`h-4 w-4 ${busy === pr.number ? 'animate-pulse' : ''}`} />
                {busy === pr.number ? 'Generating…' : 'Describe'}
              </button>
            </div>
          ))}
        </div>
      )}

      {output && (
        <div className="rounded-xl border border-gray-800 bg-gray-950 p-4">
          <p className="mb-2 text-xs text-gray-500">Generated body for #{output.number}:</p>
          <pre className="whitespace-pre-wrap break-words text-sm text-gray-300">{output.body}</pre>
        </div>
      )}
    </div>
  )
}
