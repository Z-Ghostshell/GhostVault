import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom'
import AppShell from './layout/AppShell'
import ActivityPage from './pages/ActivityPage'
import ApiConsolePage from './pages/ApiConsolePage'
import ConfigPage from './pages/ConfigPage'
import OverviewPage from './pages/OverviewPage'
import RetrieveDebugPage from './pages/RetrieveDebugPage'
import UnlockPage from './pages/UnlockPage'

export default function App() {
  const raw = import.meta.env.BASE_URL ?? '/'
  const basename = raw === '/' ? undefined : raw.replace(/\/$/, '')
  return (
    <BrowserRouter basename={basename}>
      <Routes>
        <Route path="/" element={<AppShell />}>
          <Route index element={<OverviewPage />} />
          <Route path="config" element={<ConfigPage />} />
          <Route path="activity" element={<ActivityPage />} />
          <Route path="api-console" element={<ApiConsolePage />} />
          <Route path="retrieve-debug" element={<RetrieveDebugPage />} />
          <Route path="unlock" element={<UnlockPage />} />
        </Route>
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </BrowserRouter>
  )
}
