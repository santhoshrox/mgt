import { Terminal, Github } from 'lucide-react'
import { auth } from '../lib/api'

export default function Login() {
  return (
    <div className="flex min-h-screen items-center justify-center bg-gray-950 p-6">
      <div className="w-full max-w-md animate-fade-in text-center">
        <div className="mx-auto mb-4 flex h-14 w-14 items-center justify-center rounded-2xl bg-brand-600">
          <Terminal className="h-7 w-7 text-white" />
        </div>
        <h1 className="text-2xl font-bold tracking-tight text-gray-100">
          Sign in to <span className="text-brand-400">mgt</span>
        </h1>
        <p className="mt-2 text-sm text-gray-400">
          A self-hosted dashboard for stacked pull requests.
        </p>

        <a
          href={auth.loginURL()}
          className="mt-8 inline-flex w-full items-center justify-center gap-2 rounded-lg bg-brand-600 px-4 py-2.5 text-sm font-semibold text-white transition-colors hover:bg-brand-500"
        >
          <Github className="h-4 w-4" />
          Sign in with GitHub
        </a>

        <p className="mt-6 text-xs text-gray-500">
          You'll be redirected to GitHub. mgt-be exchanges the OAuth code, encrypts your token, and sets a session cookie. The CLI uses a separate bearer token issued via <code className="rounded bg-gray-800 px-1 text-gray-300">mgt login</code>.
        </p>
      </div>
    </div>
  )
}
