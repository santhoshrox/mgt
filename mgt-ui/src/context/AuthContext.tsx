import { createContext, useContext, useState, useEffect, useCallback, type ReactNode } from 'react'
import { auth, APIError, type Me } from '../lib/api'

interface AuthContextType {
  user: Me | null
  loading: boolean
  error: string | null
  refresh: () => Promise<void>
  logout: () => Promise<void>
}

const AuthContext = createContext<AuthContextType>(null!)

export function useAuth() {
  return useContext(AuthContext)
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<Me | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const refresh = useCallback(async () => {
    try {
      setLoading(true)
      setError(null)
      const me = await auth.me()
      setUser(me)
    } catch (err) {
      setUser(null)
      if (err instanceof APIError && err.status !== 401) {
        setError(err.message)
      }
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    refresh()
  }, [refresh])

  const logout = useCallback(async () => {
    try {
      await auth.logout()
    } catch { /* ignore */ }
    setUser(null)
  }, [])

  return (
    <AuthContext.Provider value={{ user, loading, error, refresh, logout }}>
      {children}
    </AuthContext.Provider>
  )
}
