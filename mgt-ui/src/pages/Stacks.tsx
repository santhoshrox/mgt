import { useEffect, useState } from 'react'
import { GitBranch, GitPullRequest, ChevronRight, ExternalLink } from 'lucide-react'
import { stacks, pulls, type Stack, type PullRequest } from '../lib/api'
import { Loading, ErrorState, Empty } from '../components/PageState'
import RepoPicker from '../components/RepoPicker'
import { useRepoSelector } from '../lib/useRepo'

export default function Stacks() {
  const repo = useRepoSelector()
  const [data, setData] = useState<Stack[]>([])
  const [prsByBranch, setPrsByBranch] = useState<Record<string, PullRequest>>({})
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!repo.active) return
    setLoading(true)
    Promise.all([stacks.list(repo.active.id), pulls.list(repo.active.id, { state: 'open' })])
      .then(([sk, openPrs]) => {
        setData(sk)
        const map: Record<string, PullRequest> = {}
        openPrs.forEach(p => { map[p.head_branch] = p })
        setPrsByBranch(map)
      })
      .catch(err => setError(err instanceof Error ? err.message : 'load failed'))
      .finally(() => setLoading(false))
  }, [repo.active])

  if (repo.loading) return <Loading message="Loading repos..." />

  return (
    <div className="mx-auto max-w-5xl space-y-8 p-6 lg:p-8">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">Stacks</h1>
          <p className="mt-2 text-gray-400">All branch stacks tracked by mgt-be for this repo.</p>
        </div>
        <RepoPicker repos={repo.repos} active={repo.active} onSelect={repo.select} onSync={repo.sync} syncing={repo.loading} />
      </div>

      {!repo.active ? (
        <Empty icon={<GitBranch className="h-10 w-10" />} message="Pick a repo to view its stacks." />
      ) : loading ? (
        <Loading />
      ) : error ? (
        <ErrorState message={error} />
      ) : data.length === 0 ? (
        <Empty icon={<GitBranch className="h-10 w-10" />} message="No stacks yet — run `mgt create <branch>` to start one." />
      ) : (
        <div className="space-y-6">
          {data.map(stack => (
            <div key={stack.id} className="overflow-hidden rounded-xl border border-gray-800 bg-gray-900/50">
              <div className="flex items-center gap-3 border-b border-gray-800 px-5 py-3">
                <GitBranch className="h-4 w-4 text-brand-400" />
                <code className="text-sm text-gray-300">{stack.trunk_branch}</code>
                <span className="text-xs text-gray-600">stack #{stack.id}</span>
              </div>
              <div className="p-3">
                {stack.branches.length === 0 ? (
                  <p className="px-3 py-4 text-sm text-gray-500">Empty stack.</p>
                ) : (
                  <ol className="space-y-1">
                    {stack.branches.map(b => {
                      const pr = prsByBranch[b.name]
                      return (
                        <li key={b.id} className="flex items-center gap-2 rounded-lg px-3 py-2 hover:bg-gray-900">
                          <ChevronRight className="h-3.5 w-3.5 text-gray-600" />
                          <code className="text-sm text-gray-200">{b.name}</code>
                          <span className="text-xs text-gray-600">→ {b.parent || stack.trunk_branch}</span>
                          {pr && (
                            <a href={pr.html_url} target="_blank" rel="noopener noreferrer"
                              className="ml-auto inline-flex items-center gap-1 rounded-md bg-emerald-500/10 px-2 py-0.5 text-xs text-emerald-400 hover:bg-emerald-500/20">
                              <GitPullRequest className="h-3 w-3" />
                              #{pr.number}
                              <ExternalLink className="h-3 w-3" />
                            </a>
                          )}
                        </li>
                      )
                    })}
                  </ol>
                )}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
