import { useNavigate } from 'react-router-dom'

export function NotFoundPage() {
  const navigate = useNavigate()

  return (
    <div className="flex min-h-screen items-center justify-center bg-[var(--color-surface-base)] p-6">
      <div className="text-center">
        <div className="mb-4 text-[80px] leading-none font-extrabold text-[color-mix(in_srgb,_var(--color-primary)_30%,_transparent)]">
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
          className="cursor-pointer rounded-[10px] border-none bg-[linear-gradient(135deg,var(--color-brand-gold),var(--color-warning))] px-6 py-2.5 text-sm font-bold text-[var(--color-on-brand-gold)]"
        >
          Go to Home
        </button>
      </div>
    </div>
  )
}
