import { BrowserRouter, Route, Routes } from 'react-router-dom'
import { AuthProvider } from './auth/AuthContext'
import RequireAuth from './auth/RequireAuth'
import Layout from './components/Layout'
import AppointmentPage from './pages/AppointmentPage'
import DashboardPage from './pages/DashboardPage'
import DatenschutzPage from './pages/DatenschutzPage'
import ImpressumPage from './pages/ImpressumPage'
import LoginPage from './pages/LoginPage'
import OrderLookupPage from './pages/OrderLookupPage'
import WorkshopOrderDetailPage from './pages/WorkshopOrderDetailPage'
import WorkshopOrdersPage from './pages/WorkshopOrdersPage'

function NotFoundPage() {
  return (
    <section className="page-section">
      <h1 className="page-title">Seite nicht gefunden</h1>
      <p className="page-subtitle">
        Die aufgerufene Adresse existiert nicht. Nutzen Sie die Navigation, um zurückzukehren.
      </p>
    </section>
  )
}

export default function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <Routes>
          <Route element={<Layout />}>
            <Route index element={<OrderLookupPage />} />
            <Route path="auftrag" element={<AppointmentPage />} />
            <Route path="impressum" element={<ImpressumPage />} />
            <Route path="datenschutz" element={<DatenschutzPage />} />
            <Route path="werkstatt/login" element={<LoginPage />} />
            <Route path="werkstatt" element={<RequireAuth />}>
              <Route path="auftraege" element={<WorkshopOrdersPage />} />
              <Route path="auftraege/:number" element={<WorkshopOrderDetailPage />} />
              <Route path="dashboard" element={<DashboardPage />} />
            </Route>
            <Route path="*" element={<NotFoundPage />} />
          </Route>
        </Routes>
      </AuthProvider>
    </BrowserRouter>
  )
}
