import { useEffect, useMemo, useState } from 'react'
import { Search, GitPullRequest, ExternalLink, Inbox as InboxIcon } from 'lucide-react'
import { pulls, timeAgo, type PullRequest } from '../lib/api'
import { Loading, ErrorState, Empty } from '../components/PageState'
import RepoPicker from '../components/RepoPicker'
import { useRepoSelector } from '../lib/useRepo'

type StateFilter = 'open' | 'closed' | 'all'

export default function Inbox() {
  const repo = useRepoSelector()
  const [items, setItems] = useState<PullRequest[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [state, setState] = useState<StateFilter>('open')
  const [author, setAuthor] = useState('')
  const [q, setQ] = useState('')

  useEffect(() => {
    if (!repo.active) return
    setLoading(true)
    pulls.list(repo.active.id, { state: state === 'all' ? '' : state, author, q })
      .then(setItems)
      .catch(err => setError(err instanceof Error ? err.message : 'load failed'))
      .finally(() => setLoading(false))
  }, [repo.active, state, author, q])

  const visible = useMemo(() => items, [items])

  return (
    <div className="mx-auto max-w-6xl space-y-6 p-6 lg:p-8">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">PR Inbox</h1>
          <p className="mt-2 text-gray-400">All pull requests for the selected repo.</p>
        </div>
        <RepoPicker repos={repo.repos} active={repo.active} onSelect={repo.select} onSync={repo.sync} syncing={repo.loading} />
      </div>

      <div className="flex flex-wrap items-center gap-3 rounded-xl border border-gray-800 bg-gray-900/40 p-3">
        <div className="relative flex-1 min-w-[200px]">
          <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-gray-500" />
          <input
            value={q}
            onChange={e => setQ(e.target.value)}
            placeholder="Search title, body, author, branch..."
            className="w-full rounded-lg border border-gray-800 bg-gray-950 py-2 pl-9 pr-3 text-sm text-gray-200 outline-none focus:border-gray-700"
          />
        </div>
        <select value={state} onChange={e => setState(e.target.value as StateFilter)}
          className="rounded-lg border border-gray-800 bg-gray-950 py-2 px-3 text-sm text-gray-200">
          <option value="open">Open</option>
          <option value="closed">Closed</option>
          <option value="all">All</option>
        </select>
        <input
          value={author}
          onChange={e => setAuthor(e.target.value)}
          placeholder="author"
          className="w-32 rounded-lg border border-gray-800 bg-gray-950 py-2 px-3 text-sm text-gray-200"
        />
      </div>

      {!repo.active ? (
        <Empty icon={<InboxIcon className="h-10 w-10" />} message="Pick a repo to view its inbox." />
      ) : loading ? (
        <Loading />
      ) : error ? (
        <ErrorState message={error} />
      ) : visible.length === 0 ? (
        <Empty icon={<InboxIcon className="h-10 w-10" />} message="No pull requests match the current filter." />
      ) : (
        <div className="space-y-2">
          {visible.map(pr => (
            <a key={pr.number} href={pr.html_url} target="_blank" rel="noopener noreferrer"
              className="flex items-center gap-4 rounded-xl border border-gray-800 bg-gray-900/50 p-4 transition-all hover:border-gray-700 hover:bg-gray-900">
              <GitPullRequest className={`h-4 w-4 ${pr.merged ? 'text-purple-400' : pr.state === 'closed' ? 'text-rose-400' : pr.draft ? 'text-gray-500' : 'text-emerald-400'}`} />
              <div className="min-w-0 flex-1">
                <p className="truncate text-sm font-medium text-gray-200">{pr.title}</p>
                <p className="mt-0.5 text-xs text-gray-500">
                  #{pr.number} · {pr.author_login} · {pr.head_branch} → {pr.base_branch} · {timeAgo(pr.updated_at)}
                </p>
              </div>
              <div className="flex items-center gap-2 text-xs">
                <span className="text-emerald-400">+{pr.additions}</span>
                <span className="text-rose-400">-{pr.deletions}</span>
                <ExternalLink className="ml-2 h-3.5 w-3.5 text-gray-600" />
              </div>
            </a>
          ))}
        </div>
      )}
    </div>
  )
}
