import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { LoginPage } from './pages/LoginPage'
import { GroupsPage } from './pages/GroupsPage'
import { DashboardPage } from './pages/DashboardPage'
import { DistributionsPage } from './pages/DistributionsPage'
import { OrdersPage } from './pages/OrdersPage'
import { FinancesPage } from './pages/FinancesPage'
import { MembersPage } from './pages/MembersPage'
import { CatalogsPage } from './pages/CatalogsPage'
import { ProfilePage } from './pages/ProfilePage'
import { AdminPage } from './pages/AdminPage'
import { useAuthStore } from './store/auth'

const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: 1, staleTime: 30_000 } },
})

function Protected({ children }: { children: React.ReactElement }) {
  const token = useAuthStore((s) => s.token)
  return token ? children : <Navigate to="/login" replace />
}

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <Routes>
          <Route path="/login" element={<LoginPage />} />
          <Route path="/groups" element={<Protected><GroupsPage /></Protected>} />
          <Route path="/profile" element={<Protected><ProfilePage /></Protected>} />
          <Route path="/groups/:groupId" element={<Protected><DashboardPage /></Protected>} />
          <Route path="/groups/:groupId/distributions" element={<Protected><DistributionsPage /></Protected>} />
          <Route path="/groups/:groupId/orders" element={<Protected><OrdersPage /></Protected>} />
          <Route path="/groups/:groupId/finances" element={<Protected><FinancesPage /></Protected>} />
          <Route path="/groups/:groupId/members" element={<Protected><MembersPage /></Protected>} />
          <Route path="/groups/:groupId/catalogs" element={<Protected><CatalogsPage /></Protected>} />
          <Route path="/groups/:groupId/admin" element={<Protected><AdminPage /></Protected>} />
          <Route path="*" element={<Navigate to="/groups" replace />} />
        </Routes>
      </BrowserRouter>
    </QueryClientProvider>
  )
}
