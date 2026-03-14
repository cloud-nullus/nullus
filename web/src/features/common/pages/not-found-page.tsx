import { useNavigate } from 'react-router-dom'

export function NotFoundPage() {
  const navigate = useNavigate()

  return (
    <div
      style={{
        minHeight: '100vh',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        background: 'var(--color-surface-base)',
        padding: '24px',
      }}
    >
      <div style={{ textAlign: 'center' }}>
        <div
          style={{
            fontSize: '80px',
            fontWeight: 800,
            color: 'rgba(99,102,241,0.3)',
            lineHeight: 1,
            marginBottom: '16px',
          }}
        >
          404
        </div>
        <h1
          style={{
            margin: '0 0 8px',
            fontSize: '24px',
            fontWeight: 700,
            color: 'var(--color-text-primary)',
          }}
        >
          Page not found
        </h1>
        <p
          style={{
            margin: '0 0 28px',
            fontSize: '14px',
            color: 'var(--color-text-secondary)',
          }}
        >
          The page you are looking for does not exist or has been moved.
        </p>
        <button
          onClick={() => navigate('/')}
          style={{
            background: 'linear-gradient(135deg, #ffd700, #f59e0b)',
            color: '#1a1d29',
            border: 'none',
            borderRadius: '10px',
            padding: '10px 24px',
            fontSize: '14px',
            fontWeight: 700,
            cursor: 'pointer',
          }}
        >
          Go to Home
        </button>
      </div>
    </div>
  )
}
