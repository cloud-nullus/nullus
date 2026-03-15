import { useNavigate } from 'react-router-dom'

export function NotFoundPage() {
  const navigate = useNavigate()

  return (
    <div className="flex min-h-screen items-center justify-center bg-[var(--color-surface-base)] p-6">
      <div className="text-center">
        <div className="mb-4 text-[80px] leading-none font-extrabold text-[rgba(99,102,241,0.3)]">
          404
        </div>
        <h1 className="mb-2 mt-0 text-2xl font-bold text-[var(--color-text-primary)]">
          Page not found
        </h1>
        <p className="mb-7 mt-0 text-sm text-[var(--color-text-secondary)]">
          The page you are looking for does not exist or has been moved.
        </p>
        <button
          type="button"
          onClick={() => navigate('/')}
          className="cursor-pointer rounded-[10px] border-none bg-[linear-gradient(135deg,#ffd700,#f59e0b)] px-6 py-2.5 text-sm font-bold text-[#1a1d29]"
        >
          Go to Home
        </button>
      </div>
    </div>
  )
}
