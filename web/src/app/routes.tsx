import { createBrowserRouter } from 'react-router-dom'
import { type ReactNode, lazy, Suspense } from 'react'
import { AppLayout } from './layout'
import { ProtectedRoute } from '../components/shared/protected-route'

const LoginPage = lazy(() =>
  import('../features/auth/pages/login-page').then((m) => ({ default: m.LoginPage }))
)

const NotFoundPage = lazy(() =>
  import('../features/common/pages/not-found-page').then((m) => ({ default: m.NotFoundPage }))
)

const HomePage = lazy(() =>
  import('../features/home/pages/home-page').then((m) => ({ default: m.HomePage }))
)

const StackTemplatePage = lazy(() =>
  import('../features/stack/pages/stack-template-page').then((m) => ({ default: m.StackTemplatePage }))
)

const StackInstallPage = lazy(() =>
  import('../features/stack/pages/stack-install-page').then((m) => ({ default: m.StackInstallPage }))
)

const StackAddToolsPage = lazy(() =>
  import('../features/stack/pages/stack-add-tools-page').then((m) => ({ default: m.StackAddToolsPage }))
)

const StackListPage = lazy(() =>
  import('../features/stack/pages/stack-list-page').then((m) => ({ default: m.StackListPage }))
)

const StackDeployPage = lazy(() =>
  import('../features/stack/pages/stack-deploy-page').then((m) => ({ default: m.StackDeployPage }))
)

const StackHistoryPage = lazy(() =>
  import('../features/stack/pages/stack-history-page').then((m) => ({ default: m.StackHistoryPage }))
)


const StackVersionPage = lazy(() =>
  import('../features/stack/pages/stack-version-page').then((m) => ({ default: m.StackVersionPage }))
)

const StackOssResourceDefaultPage = lazy(() =>
  import('../features/stack/pages/stack-oss-resource-default-page').then((m) => ({ default: m.StackOssResourceDefaultPage }))
)

const DeveloperDeployPage = lazy(() =>
  import('../features/cicd/pages/developer-deploy-page').then((m) => ({ default: m.DeveloperDeployPage }))
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

const KnownIssuesPage = lazy(() =>
  import('../features/admin/pages/known-issues-page').then((m) => ({ default: m.KnownIssuesPage }))
)

function Loading() {
  return (
    <div className="flex h-[200px] items-center justify-center text-[var(--color-text-secondary)]">
      Loading...
    </div>
  )
}

function withSuspense(element: ReactNode) {
  return <Suspense fallback={<Loading />}>{element}</Suspense>
}

export const router = createBrowserRouter([
  { path: '/login', element: withSuspense(<LoginPage />) },
  {
    path: '/',
    element: <AppLayout />,
    children: [
      {
        element: <ProtectedRoute />,
        children: [
          { index: true, element: withSuspense(<HomePage />) },
          { path: 'stack/templates', element: withSuspense(<StackTemplatePage />) },
          { path: 'stack/list', element: withSuspense(<StackListPage />) },
          { path: 'stack/logs/:deploymentId', element: withSuspense(<StackDeployPage />) },
          { path: 'stack/history/:stackId?', element: withSuspense(<StackHistoryPage />) },
          { path: 'stack/versions', element: withSuspense(<StackVersionPage />) },
          { path: 'stack/version', element: withSuspense(<StackVersionPage />) },
          { path: 'observability/monitoring', element: withSuspense(<MonitoringPage />) },
          { path: 'observability/alerts', element: withSuspense(<AlertRulesPage />) },
          { path: 'observability/alert-rules', element: withSuspense(<AlertRulesPage />) },
          { path: 'observability/alert-history', element: withSuspense(<AlertHistoryPage />) },
        ],
      },
      {
        element: <ProtectedRoute allowedRoles={['admin', 'devops']} />,
        children: [
          { path: 'stack/install', element: withSuspense(<StackInstallPage />) },
          { path: 'stack/:id/add-tools', element: withSuspense(<StackAddToolsPage />) },
          { path: 'stack/deploy/:id', element: withSuspense(<StackDeployPage />) },
          { path: 'stack/oss-resource-default', element: withSuspense(<StackOssResourceDefaultPage />) },
        ],
      },
      {
        element: <ProtectedRoute allowedRoles={['admin', 'devops', 'developer']} />,
        children: [
          { path: 'cicd/developer-deploy', element: withSuspense(<DeveloperDeployPage />) },
          { path: 'cicd/templates', element: withSuspense(<CicdTemplatePage />) },
          { path: 'cicd/create', element: withSuspense(<DeveloperDeployPage />) },
          { path: 'cicd/list', element: withSuspense(<CicdListPage />) },
          { path: 'cicd/history', element: withSuspense(<CicdHistoryPage />) },
        ],
      },
      {
        element: <ProtectedRoute allowedRoles={['admin']} />,
        children: [
          { path: 'admin/organization', element: withSuspense(<OrganizationPage />) },
          { path: 'admin/organizations', element: withSuspense(<OrganizationPage />) },
          { path: 'admin/users', element: withSuspense(<UserManagementPage />) },
          { path: 'admin/clusters', element: withSuspense(<ClusterPage />) },
          { path: 'admin/known-issues', element: withSuspense(<KnownIssuesPage />) },
        ],
      },
      { path: '*', element: withSuspense(<NotFoundPage />) },
    ],
  },
])
