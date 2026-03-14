import { createBrowserRouter, Navigate } from 'react-router-dom'
import { type ReactNode, lazy, Suspense } from 'react'
import { AppLayout } from './layout'

const HomePage = lazy(() =>
  import('../features/home/pages/home-page').then((m) => ({ default: m.HomePage }))
)

const StackTemplatePage = lazy(() =>
  import('../features/stack/pages/stack-template-page').then((m) => ({ default: m.StackTemplatePage }))
)

const StackInstallPage = lazy(() =>
  import('../features/stack/pages/stack-install-page').then((m) => ({ default: m.StackInstallPage }))
)

const StackListPage = lazy(() =>
  import('../features/stack/pages/stack-list-page').then((m) => ({ default: m.StackListPage }))
)

const OrganizationPage = lazy(() =>
  import('../features/admin/pages/organization-page').then((m) => ({ default: m.OrganizationPage }))
)

const ClusterPage = lazy(() =>
  import('../features/admin/pages/cluster-page').then((m) => ({ default: m.ClusterPage }))
)

function Loading() {
  return (
    <div
      style={{
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        height: '200px',
        color: 'var(--color-text-secondary)',
      }}
    >
      Loading...
    </div>
  )
}

function withSuspense(element: ReactNode) {
  return <Suspense fallback={<Loading />}>{element}</Suspense>
}

export const router = createBrowserRouter([
  {
    path: '/',
    element: <AppLayout />,
    children: [
      { index: true, element: withSuspense(<HomePage />) },
      { path: 'stack/templates', element: withSuspense(<StackTemplatePage />) },
      { path: 'stack/install', element: withSuspense(<StackInstallPage />) },
      { path: 'stack/list', element: withSuspense(<StackListPage />) },
      { path: 'stack/history', element: withSuspense(<HomePage />) },
      { path: 'stack/version', element: withSuspense(<HomePage />) },
      { path: 'cicd/templates', element: withSuspense(<HomePage />) },
      { path: 'cicd/list', element: withSuspense(<HomePage />) },
      { path: 'cicd/history', element: withSuspense(<HomePage />) },
      { path: 'observability/monitoring', element: withSuspense(<HomePage />) },
      { path: 'observability/alerts', element: withSuspense(<HomePage />) },
      { path: 'observability/alert-history', element: withSuspense(<HomePage />) },
      { path: 'admin/organization', element: withSuspense(<OrganizationPage />) },
      { path: 'admin/users', element: withSuspense(<HomePage />) },
      { path: 'admin/clusters', element: withSuspense(<ClusterPage />) },
      { path: '*', element: <Navigate to="/" replace /> },
    ],
  },
])
