import { useEffect, useState } from 'react'
import { repos, type Repo } from './api'

const KEY = 'mgt_active_repo_id'

// useRepoSelector loads the registered repos and tracks which one is active
// in localStorage. Most pages need exactly one repo at a time; the picker
// surfaces the rest.
export function useRepoSelector() {
  const [list, setList] = useState<Repo[]>([])
  const [active, setActive] = useState<Repo | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  async function load(sync = false) {
    try {
      setLoading(true)
      setError(null)
      const fetched = sync ? await repos.sync() : await repos.list()
      setList(fetched)
      const stored = Number(localStorage.getItem(KEY))
      const next = fetched.find(r => r.id === stored) ?? fetched[0] ?? null
      setActive(next)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load repos')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load()
  }, [])

  function select(repo: Repo) {
    setActive(repo)
    localStorage.setItem(KEY, String(repo.id))
  }

  return { repos: list, active, loading, error, select, refresh: () => load(false), sync: () => load(true) }
}
