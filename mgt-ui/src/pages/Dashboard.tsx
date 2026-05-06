import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import {
  GitBranch, Inbox, Bot, ListOrdered, BarChart3, ArrowRight,
  GitPullRequest, ExternalLink,
} from 'lucide-react'
import { useAuth } from '../context/AuthContext'
import { pulls, timeAgo, type PullRequest } from '../lib/api'
import { Loading, ErrorState } from '../components/PageState'
import RepoPicker from '../components/RepoPicker'
import { useRepoSelector } from '../lib/useRepo'
import { mgtCommands } from '../data/mock'

const features = [
  { title: 'Stacks', desc: 'Visualise stacked branches.', icon: GitBranch, to: '/stacks', color: 'text-brand-400', bg: 'bg-brand-500/10' },
  { title: 'PR Inbox', desc: 'Search and filter your PRs.', icon: Inbox, to: '/inbox', color: 'text-emerald-glow', bg: 'bg-emerald-500/10' },
  { title: 'AI Reviews', desc: 'Server-side LLM-powered review summaries.', icon: Bot, to: '/reviews', color: 'text-amber-glow', bg: 'bg-amber-500/10' },
  { title: 'Merge Queue', desc: 'Track the FIFO merge queue.', icon: ListOrdered, to: '/merge-queue', color: 'text-rose-glow', bg: 'bg-rose-500/10' },
  { title: 'Insights', desc: 'Throughput and cycle-time metrics.', icon: BarChart3, to: '/insights', color: 'text-cyan-400', bg: 'bg-cyan-500/10' },
]

export default function Dashboard() {
  const { user } = useAuth()
  const repo = useRepoSelector()
  const [openPRs, setOpenPRs] = useState<PullRequest[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!repo.active) return
    setLoading(true)
    pulls.list(repo.active.id, { state: 'open' })
      .then(setOpenPRs)
      .catch(err => setError(err instanceof Error ? err.message : 'load failed'))
      .finally(() => setLoading(false))
  }, [repo.active])

  if (repo.loading) return <Loading message="Loading repos..." />
  if (repo.error) return <ErrorState message={repo.error} onRetry={repo.refresh} />

  const mine = openPRs.filter(p => p.author_login === user?.login)
  const others = openPRs.filter(p => p.author_login !== user?.login)

  return (
    <div className="mx-auto max-w-7xl space-y-8 p-6 lg:p-8">
      <div className="flex flex-wrap items-start justify-between gap-4 animate-fade-in">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">
            Welcome, <span className="text-brand-400">{user?.login}</span>
          </h1>
          <p className="mt-2 text-gray-400">Self-hosted graphite, backed by mgt-be.</p>
        </div>
        <RepoPicker repos={repo.repos} active={repo.active} onSelect={repo.select} onSync={repo.sync} syncing={repo.loading} />
      </div>

      {!repo.active ? (
        <p className="rounded-xl border border-gray-800 bg-gray-900/40 p-6 text-sm text-gray-400">
          No repositories registered yet. Click <strong>Sync</strong> above to import your GitHub repos.
        </p>
      ) : (
        <>
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
            <Stat label="Open PRs (this repo)" value={openPRs.length} />
            <Stat label="Yours" value={mine.length} />
            <Stat label="Others" value={others.length} />
            <Stat label="Drafts" value={openPRs.filter(p => p.draft).length} />
          </div>

          {loading ? <Loading /> : error ? <ErrorState message={error} /> : (
            <div className="grid gap-6 lg:grid-cols-2">
              <PRList title="Your open PRs" items={mine.slice(0, 6)} />
              <PRList title="Others' open PRs" items={others.slice(0, 6)} />
            </div>
          )}
        </>
      )}

      <div>
        <h2 className="mb-4 text-lg font-semibold">Features</h2>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {features.map(({ title, desc, icon: Icon, to, color, bg }) => (
            <Link key={to} to={to} className="group rounded-xl border border-gray-800 bg-gray-900/50 p-6 transition-all hover:border-gray-700 hover:bg-gray-900">
              <div className={`inline-flex rounded-lg p-2.5 ${bg}`}><Icon className={`h-5 w-5 ${color}`} /></div>
              <h3 className="mt-4 font-semibold text-gray-100">{title}</h3>
              <p className="mt-2 text-sm leading-relaxed text-gray-400">{desc}</p>
              <div className="mt-4 flex items-center gap-1 text-sm font-medium text-brand-400 opacity-0 transition-opacity group-hover:opacity-100">
                Explore <ArrowRight className="h-4 w-4" />
              </div>
            </Link>
          ))}
        </div>
      </div>

      <div>
        <h2 className="mb-4 text-lg font-semibold">CLI reference</h2>
        <div className="overflow-hidden rounded-xl border border-gray-800 bg-gray-950">
          <div className="divide-y divide-gray-800/50">
            {mgtCommands.map(({ cmd, desc }) => (
              <div key={cmd} className="flex items-start gap-4 px-4 py-3">
                <code className="min-w-[12rem] shrink-0 text-sm font-medium text-brand-400">$ {cmd}</code>
                <span className="text-sm text-gray-400">{desc}</span>
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  )
}

function Stat({ label, value }: { label: string; value: number }) {
  return (
    <div className="rounded-xl border border-gray-800 bg-gray-900/50 p-5">
      <p className="text-sm text-gray-400">{label}</p>
      <p className="mt-1 text-2xl font-bold">{value}</p>
    </div>
  )
}

function PRList({ title, items }: { title: string; items: PullRequest[] }) {
  return (
    <div>
      <h2 className="mb-3 text-lg font-semibold">{title}</h2>
      {items.length === 0 ? (
        <p className="rounded-xl border border-gray-800 bg-gray-900/40 p-6 text-sm text-gray-500">Nothing here.</p>
      ) : (
        <div className="space-y-2">
          {items.map(pr => (
            <a key={pr.number} href={pr.html_url} target="_blank" rel="noopener noreferrer"
              className="flex items-center gap-3 rounded-xl border border-gray-800 bg-gray-900/50 p-4 transition-all hover:border-gray-700 hover:bg-gray-900">
              <GitPullRequest className={`h-4 w-4 ${pr.draft ? 'text-gray-500' : 'text-emerald-400'}`} />
              <div className="min-w-0 flex-1">
                <p className="truncate text-sm font-medium text-gray-200">{pr.title}</p>
                <p className="mt-0.5 text-xs text-gray-500">#{pr.number} · {pr.author_login} · {timeAgo(pr.updated_at)}</p>
              </div>
              <ExternalLink className="h-3.5 w-3.5 text-gray-600" />
            </a>
          ))}
        </div>
      )}
    </div>
  )
}
