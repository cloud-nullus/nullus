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

const StackDeployPage = lazy(() =>
  import('../features/stack/pages/stack-deploy-page').then((m) => ({ default: m.StackDeployPage }))
)

const CicdTemplatePage = lazy(() =>
  import('../features/cicd/pages/cicd-template-page').then((m) => ({ default: m.CicdTemplatePage }))
)

const CicdListPage = lazy(() =>
  import('../features/cicd/pages/cicd-list-page').then((m) => ({ default: m.CicdListPage }))
)

const CicdHistoryPage = lazy(() =>
  import('../features/cicd/pages/cicd-history-page').then((m) => ({ default: m.CicdHistoryPage }))
)

const MonitoringPage = lazy(() =>
  import('../features/observability/pages/monitoring-page').then((m) => ({ default: m.MonitoringPage }))
)

const AlertRulesPage = lazy(() =>
  import('../features/observability/pages/alert-rules-page').then((m) => ({ default: m.AlertRulesPage }))
)

const AlertHistoryPage = lazy(() =>
  import('../features/observability/pages/alert-history-page').then((m) => ({ default: m.AlertHistoryPage }))
)

const OrganizationPage = lazy(() =>
  import('../features/admin/pages/organization-page').then((m) => ({ default: m.OrganizationPage }))
)

const ClusterPage = lazy(() =>
  import('../features/admin/pages/cluster-page').then((m) => ({ default: m.ClusterPage }))
)

const UserManagementPage = lazy(() =>
  import('../features/admin/pages/user-management-page').then((m) => ({ default: m.UserManagementPage }))
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
      { path: 'stack/deploy/:id', element: withSuspense(<StackDeployPage />) },
      { path: 'cicd/templates', element: withSuspense(<CicdTemplatePage />) },
      { path: 'cicd/list', element: withSuspense(<CicdListPage />) },
      { path: 'cicd/history', element: withSuspense(<CicdHistoryPage />) },
      { path: 'observability/monitoring', element: withSuspense(<MonitoringPage />) },
      { path: 'observability/alerts', element: withSuspense(<AlertRulesPage />) },
      { path: 'observability/alert-rules', element: withSuspense(<AlertRulesPage />) },
      { path: 'observability/alert-history', element: withSuspense(<AlertHistoryPage />) },
      { path: 'admin/organization', element: withSuspense(<OrganizationPage />) },
      { path: 'admin/users', element: withSuspense(<UserManagementPage />) },
      { path: 'admin/clusters', element: withSuspense(<ClusterPage />) },
      { path: '*', element: <Navigate to="/" replace /> },
    ],
  },
])
