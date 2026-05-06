import { ChevronDown, RefreshCw } from 'lucide-react'
import type { Repo } from '../lib/api'

interface Props {
  repos: Repo[]
  active: Repo | null
  onSelect: (r: Repo) => void
  onSync: () => void
  syncing?: boolean
}

export default function RepoPicker({ repos, active, onSelect, onSync, syncing }: Props) {
  return (
    <div className="flex items-center gap-2">
      <div className="relative">
        <select
          value={active?.id ?? ''}
          onChange={e => {
            const r = repos.find(r => r.id === Number(e.target.value))
            if (r) onSelect(r)
          }}
          className="appearance-none rounded-lg border border-gray-800 bg-gray-900 py-1.5 pl-3 pr-8 text-sm text-gray-200"
        >
          {repos.length === 0 && <option value="">No repos</option>}
          {repos.map(r => (
            <option key={r.id} value={r.id}>{r.full_name}</option>
          ))}
        </select>
        <ChevronDown className="pointer-events-none absolute right-2 top-1/2 h-4 w-4 -translate-y-1/2 text-gray-500" />
      </div>
      <button
        onClick={onSync}
        disabled={syncing}
        className="inline-flex items-center gap-1.5 rounded-lg border border-gray-800 bg-gray-900 px-3 py-1.5 text-xs text-gray-300 hover:border-gray-700 disabled:opacity-50"
        title="Refresh from GitHub"
      >
        <RefreshCw className={`h-3.5 w-3.5 ${syncing ? 'animate-spin' : ''}`} />
        Sync
      </button>
    </div>
  )
}
