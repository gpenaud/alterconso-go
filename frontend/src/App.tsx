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
import { ShopPage } from './pages/ShopPage'
// Aperçu de la refonte, hors parcours : voir docs/refonte-front.md.
import { AperçuRefonte } from './pages/Refonte/AperçuRefonte'
import { AccueilConnecte } from './pages/Refonte/AccueilConnecte'
import { DistributionsConnecte } from './pages/Refonte/DistributionsConnecte'
import { MesCommandes } from './pages/Refonte/MesCommandes'
import { CompteRefonte } from './pages/Refonte/CompteRefonte'
import { ConfirmationConnecte } from './pages/Refonte/ConfirmationConnecte'
import { ProducteurRefonte } from './pages/Refonte/ProducteurRefonte'
import { MessageRefonte } from './pages/Refonte/MessageRefonte'
import { AdminLayout } from './pages/Refonte/AdminLayout'
import { AdminBord } from './pages/Refonte/AdminBord'
import { AdminCommandes } from './pages/Refonte/AdminCommandes'
import { AdminDistributions } from './pages/Refonte/AdminDistributions'
import { AdminCatalogues } from './pages/Refonte/AdminCatalogues'
import { AdminMembres } from './pages/Refonte/AdminMembres'
import { AdminDroits } from './pages/Refonte/AdminDroits'
import { AdminRemise } from './pages/Refonte/AdminRemise'
import { AdminAdhesions } from './pages/Refonte/AdminAdhesions'
import { AdminMembre } from './pages/Refonte/AdminMembre'
import { LayoutRefonte } from './pages/Refonte/LayoutRefonte'
import { useAuthStore } from './store/auth'

const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: 1, staleTime: 30_000 } },
})

function Protected({ children }: { children: React.ReactElement }) {
  const token = useAuthStore((s) => s.token)
  return token ? children : <Navigate to="/login" replace />
}

import { ImpersonationBanner } from "./components/ImpersonationBanner"
import { PhoneSuggestionBanner } from "./components/PhoneSuggestionBanner"

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <ImpersonationBanner />
        <PhoneSuggestionBanner />
        <Routes>
          <Route path="/login" element={<LoginPage />} />
          <Route path="/refonte" element={<Protected><LayoutRefonte><AccueilConnecte /></LayoutRefonte></Protected>} />
          <Route path="/refonte/distributions" element={<Protected><DistributionsConnecte /></Protected>} />
          <Route path="/refonte/commandes" element={<Protected><LayoutRefonte><MesCommandes /></LayoutRefonte></Protected>} />
          <Route path="/refonte/compte" element={<Protected><LayoutRefonte><CompteRefonte /></LayoutRefonte></Protected>} />
          <Route path="/refonte/confirmation/:multiDistribId" element={<Protected><ConfirmationConnecte /></Protected>} />
          <Route path="/refonte/producteur/:vendorId" element={<Protected><LayoutRefonte><ProducteurRefonte /></LayoutRefonte></Protected>} />
          <Route path="/refonte/message" element={<Protected><LayoutRefonte><MessageRefonte /></LayoutRefonte></Protected>} />
          <Route path="/refonte/admin" element={<Protected><AdminLayout><AdminBord /></AdminLayout></Protected>} />
          <Route path="/refonte/admin/commandes" element={<Protected><AdminLayout><AdminCommandes /></AdminLayout></Protected>} />
          <Route path="/refonte/admin/distributions" element={<Protected><AdminLayout><AdminDistributions /></AdminLayout></Protected>} />
          <Route path="/refonte/admin/catalogues" element={<Protected><AdminLayout><AdminCatalogues /></AdminLayout></Protected>} />
          <Route path="/refonte/admin/membres" element={<Protected><AdminLayout><AdminMembres /></AdminLayout></Protected>} />
          <Route path="/refonte/admin/droits" element={<Protected><AdminLayout><AdminDroits /></AdminLayout></Protected>} />
          <Route path="/refonte/admin/remise" element={<Protected><AdminRemise /></Protected>} />
          <Route path="/refonte/admin/adhesions" element={<Protected><AdminLayout><AdminAdhesions /></AdminLayout></Protected>} />
          <Route path="/refonte/admin/membres/:memberId" element={<Protected><AdminLayout><AdminMembre /></AdminLayout></Protected>} />
          <Route path="/refonte/apercu" element={<AperçuRefonte />} />
          <Route path="/groups" element={<Protected><GroupsPage /></Protected>} />
          <Route path="/profile" element={<Protected><ProfilePage /></Protected>} />
          <Route path="/groups/:groupId" element={<Protected><DashboardPage /></Protected>} />
          <Route path="/groups/:groupId/distributions" element={<Protected><DistributionsPage /></Protected>} />
          <Route path="/groups/:groupId/orders" element={<Protected><OrdersPage /></Protected>} />
          <Route path="/groups/:groupId/finances" element={<Protected><FinancesPage /></Protected>} />
          <Route path="/groups/:groupId/members" element={<Protected><MembersPage /></Protected>} />
          <Route path="/groups/:groupId/catalogs" element={<Protected><CatalogsPage /></Protected>} />
          <Route path="/groups/:groupId/admin" element={<Protected><AdminPage /></Protected>} />
          {/* Shop accessible directement : l'auth API se fait via le cookie de
              session JWT déjà en place (middleware Auth accepte cookie OU Bearer). */}
          <Route path="/shop/:multiDistribId" element={<ShopPage />} />
          <Route path="*" element={<Navigate to="/groups" replace />} />
        </Routes>
      </BrowserRouter>
    </QueryClientProvider>
  )
}
