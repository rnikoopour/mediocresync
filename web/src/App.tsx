import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { Layout } from './components/Layout'
import { ConnectionsPage } from './pages/ConnectionsPage'
import { JobsPage } from './pages/JobsPage'
import { JobDetailPage } from './pages/JobDetailPage'
import { RunDetailPage } from './pages/RunDetailPage'
import { LoginPage } from './pages/LoginPage'
import { SetupPage } from './pages/SetupPage'
import { SettingsGeneralPage } from './pages/SettingsGeneralPage'
import { SettingsIndexPage } from './pages/SettingsIndexPage'
import { LogsPage } from './pages/LogsPage'
import { PlanProvider } from './context/PlanContext'

const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: 1, staleTime: 10_000 } },
})

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <PlanProvider>
      <BrowserRouter>
        <Routes>
          <Route path="login" element={<LoginPage />} />
          <Route path="setup" element={<SetupPage />} />
          <Route element={<Layout />}>
            <Route index element={<Navigate to="/jobs" replace />} />
            <Route path="connections" element={<ConnectionsPage />} />
            <Route path="jobs" element={<JobsPage />} />
            <Route path="jobs/:id" element={<JobDetailPage />} />
            <Route path="runs/:id" element={<RunDetailPage />} />
            <Route path="settings" element={<SettingsIndexPage />} />
            <Route path="settings/general" element={<SettingsGeneralPage />} />
            <Route path="settings/logs" element={<LogsPage />} />
          </Route>
        </Routes>
      </BrowserRouter>
      </PlanProvider>
    </QueryClientProvider>
  )
}
