import { useEffect, useMemo, useState } from 'react'
import { BarChart3 } from 'lucide-react'
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer } from 'recharts'
import { pulls, type PullRequest } from '../lib/api'
import { Loading, ErrorState, Empty } from '../components/PageState'
import RepoPicker from '../components/RepoPicker'
import { useRepoSelector } from '../lib/useRepo'

// hoursBetween returns the difference between two timestamps in fractional hours.
function hoursBetween(a: string, b: string): number {
  return Math.abs(new Date(a).getTime() - new Date(b).getTime()) / 3_600_000
}

export default function Insights() {
  const repo = useRepoSelector()
  const [items, setItems] = useState<PullRequest[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!repo.active) return
    setLoading(true)
    pulls.list(repo.active.id, { state: 'all' })
      .then(setItems)
      .catch(err => setError(err instanceof Error ? err.message : 'load failed'))
      .finally(() => setLoading(false))
  }, [repo.active])

  const stats = useMemo(() => {
    const merged = items.filter(p => p.merged)
    const open = items.filter(p => p.state === 'open')
    const cycleHours = merged.length
      ? merged.reduce((sum, p) => sum + hoursBetween(p.updated_at, p.updated_at), 0) / merged.length
      : 0

    // Group merges by week (yyyy-ww-ish bucket).
    const buckets: Record<string, number> = {}
    merged.forEach(p => {
      const d = new Date(p.updated_at)
      const key = `${d.getFullYear()}-${String(Math.ceil((d.getDate()) / 7)).padStart(2, '0')}-${String(d.getMonth() + 1).padStart(2, '0')}`
      buckets[key] = (buckets[key] ?? 0) + 1
    })
    const series = Object.entries(buckets)
      .sort(([a], [b]) => a.localeCompare(b))
      .slice(-8)
      .map(([k, v]) => ({ week: k, merged: v }))

    return { open: open.length, merged: merged.length, cycleHours, series }
  }, [items])

  return (
    <div className="mx-auto max-w-5xl space-y-6 p-6 lg:p-8">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">Insights</h1>
          <p className="mt-2 text-gray-400">Throughput metrics derived from synced PRs.</p>
        </div>
        <RepoPicker repos={repo.repos} active={repo.active} onSelect={repo.select} onSync={repo.sync} syncing={repo.loading} />
      </div>

      {!repo.active ? (
        <Empty icon={<BarChart3 className="h-10 w-10" />} message="Pick a repo to see insights." />
      ) : loading ? (
        <Loading />
      ) : error ? (
        <ErrorState message={error} />
      ) : items.length === 0 ? (
        <Empty icon={<BarChart3 className="h-10 w-10" />} message="No PRs synced yet — open one or run `mgt sync-repos` and wait for webhooks." />
      ) : (
        <>
          <div className="grid gap-4 sm:grid-cols-3">
            <Stat label="Open PRs" value={stats.open} />
            <Stat label="Merged PRs" value={stats.merged} />
            <Stat label="Avg. cycle" value={`${stats.cycleHours.toFixed(1)}h`} />
          </div>

          <div className="rounded-xl border border-gray-800 bg-gray-900/50 p-4">
            <h2 className="mb-3 text-sm font-medium text-gray-300">Merges per week</h2>
            <div className="h-64">
              <ResponsiveContainer width="100%" height="100%">
                <BarChart data={stats.series}>
                  <CartesianGrid strokeDasharray="3 3" stroke="#1f2937" />
                  <XAxis dataKey="week" stroke="#6b7280" tick={{ fontSize: 11 }} />
                  <YAxis stroke="#6b7280" tick={{ fontSize: 11 }} />
                  <Tooltip contentStyle={{ background: '#0b0d12', border: '1px solid #1f2937' }} />
                  <Bar dataKey="merged" fill="#34d399" radius={[4, 4, 0, 0]} />
                </BarChart>
              </ResponsiveContainer>
            </div>
          </div>
        </>
      )}
    </div>
  )
}

function Stat({ label, value }: { label: string; value: string | number }) {
  return (
    <div className="rounded-xl border border-gray-800 bg-gray-900/50 p-5">
      <p className="text-sm text-gray-400">{label}</p>
      <p className="mt-1 text-2xl font-bold">{value}</p>
    </div>
  )
}
