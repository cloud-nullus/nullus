import { type ErrorInfo, type ReactNode, Component } from 'react'

interface ErrorBoundaryProps {
  children: ReactNode
}

interface ErrorBoundaryState {
  hasError: boolean
  error: Error | null
}

export class ErrorBoundary extends Component<ErrorBoundaryProps, ErrorBoundaryState> {
  public constructor(props: ErrorBoundaryProps) {
    super(props)
    this.state = {
      hasError: false,
      error: null,
    }
  }

  public static getDerivedStateFromError(error: Error): ErrorBoundaryState {
    return {
      hasError: true,
      error,
    }
  }

  public componentDidCatch(error: Error, errorInfo: ErrorInfo) {
    console.error('Unhandled render error:', error, errorInfo)
  }

  private resetErrorBoundary = () => {
    this.setState({ hasError: false, error: null })
  }

  public render() {
    if (this.state.hasError) {
      return (
        <div className="flex min-h-[50vh] items-center justify-center p-6">
          <div className="w-full max-w-[560px] rounded-[14px] border border-[var(--color-border-default)] bg-[var(--color-surface-card)] p-6">
            <h2 className="mb-2.5 mt-0 text-xl font-extrabold text-[var(--color-text-primary)]">
              Something went wrong
            </h2>
            <p className="mb-3.5 mt-0 text-sm text-[var(--color-text-secondary)]">
              A rendering error interrupted this page.
            </p>
            <div className="mb-4 break-words rounded-lg border border-[rgba(239,68,68,0.25)] bg-[rgba(239,68,68,0.08)] px-3 py-2.5 font-mono text-[13px] text-[#f87171]">
              {this.state.error?.message ?? 'Unknown error'}
            </div>
            <div className="flex gap-2.5">
              <button
                type="button"
                onClick={this.resetErrorBoundary}
                className="cursor-pointer rounded-[10px] border-none bg-[linear-gradient(135deg,var(--color-brand-gold),var(--color-brand-gold-end))] px-4 py-2.5 text-sm font-bold text-[#1a1d29]"
              >
                Try Again
              </button>
              <button
                type="button"
                onClick={() => {
                  window.location.href = '/'
                }}
                className="cursor-pointer rounded-[10px] border border-[var(--color-border-default)] bg-transparent px-4 py-2.5 text-sm font-semibold text-[var(--color-text-primary)]"
              >
                Go Home
              </button>
            </div>
          </div>
        </div>
      )
    }

    return this.props.children
  }
}
