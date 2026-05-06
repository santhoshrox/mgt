import { Routes, Route, Navigate } from 'react-router-dom'
import { AuthProvider, useAuth } from './context/AuthContext'
import Layout from './components/Layout'
import Login from './pages/Login'
import Dashboard from './pages/Dashboard'
import Stacks from './pages/Stacks'
import Inbox from './pages/Inbox'
import AIReviews from './pages/AIReviews'
import MergeQueue from './pages/MergeQueue'
import Insights from './pages/Insights'
import { Loader2 } from 'lucide-react'

function FullScreenLoader() {
  return (
    <div className="flex h-screen items-center justify-center bg-gray-950">
      <Loader2 className="h-8 w-8 animate-spin text-brand-500" />
    </div>
  )
}

function AuthGate() {
  const { user, loading } = useAuth()
  if (loading) return <FullScreenLoader />
  if (!user) return <Navigate to="/login" replace />
  return <Layout />
}

function AppRoutes() {
  const { user, loading } = useAuth()
  if (loading) return <FullScreenLoader />

  return (
    <Routes>
      <Route path="/login" element={user ? <Navigate to="/" replace /> : <Login />} />
      <Route element={<AuthGate />}>
        <Route index element={<Dashboard />} />
        <Route path="stacks" element={<Stacks />} />
        <Route path="inbox" element={<Inbox />} />
        <Route path="reviews" element={<AIReviews />} />
        <Route path="merge-queue" element={<MergeQueue />} />
        <Route path="insights" element={<Insights />} />
      </Route>
    </Routes>
  )
}

export default function App() {
  return (
    <AuthProvider>
      <AppRoutes />
    </AuthProvider>
  )
}
