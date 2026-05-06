import { Loader2, AlertCircle, RefreshCw } from 'lucide-react'

export function Loading({ message = 'Loading...' }: { message?: string }) {
  return (
    <div className="flex h-full min-h-[400px] flex-col items-center justify-center gap-3">
      <Loader2 className="h-8 w-8 animate-spin text-brand-500" />
      <p className="text-sm text-gray-400">{message}</p>
    </div>
  )
}

export function ErrorState({ message, onRetry }: { message: string; onRetry?: () => void }) {
  return (
    <div className="flex h-full min-h-[400px] flex-col items-center justify-center gap-4">
      <AlertCircle className="h-10 w-10 text-rose-400/60" />
      <div className="text-center">
        <p className="text-sm font-medium text-gray-300">Something went wrong</p>
        <p className="mt-1 text-xs text-gray-500">{message}</p>
      </div>
      {onRetry && (
        <button
          onClick={onRetry}
          className="inline-flex items-center gap-2 rounded-lg border border-gray-700 px-4 py-2 text-sm text-gray-300 transition-colors hover:border-gray-600 hover:text-white"
        >
          <RefreshCw className="h-4 w-4" />
          Retry
        </button>
      )}
    </div>
  )
}

export function Empty({ icon, message }: { icon: React.ReactNode; message: string }) {
  return (
    <div className="flex h-full min-h-[300px] flex-col items-center justify-center gap-3 text-gray-600">
      {icon}
      <p className="text-sm">{message}</p>
    </div>
  )
}
