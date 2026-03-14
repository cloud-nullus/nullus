import { Outlet } from 'react-router-dom'
import { Sidebar } from '../components/layout/sidebar'
import { Header } from '../components/layout/header'

export function AppLayout() {
  return (
    <div style={{ display: 'flex', minHeight: '100vh' }}>
      <Sidebar />
      <div style={{ flex: 1, display: 'flex', flexDirection: 'column', minWidth: 0 }}>
        <Header />
        <main
          style={{
            flex: 1,
            padding: '32px var(--page-padding)',
            overflowY: 'auto',
          }}
        >
          <Outlet />
        </main>
      </div>
    </div>
  )
}
