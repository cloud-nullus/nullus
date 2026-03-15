import { Outlet } from 'react-router-dom'
import { Sidebar } from '../components/layout/sidebar'
import { Header } from '../components/layout/header'
import { ErrorBoundary } from '../components/shared/error-boundary'

export function AppLayout() {
  return (
    <div className="flex min-h-screen">
      <Sidebar />
      <div className="flex min-w-0 flex-1 flex-col">
        <Header />
        <main className="flex-1 overflow-y-auto px-[var(--page-padding)] py-8">
          <ErrorBoundary>
            <Outlet />
          </ErrorBoundary>
        </main>
      </div>
    </div>
  )
}
