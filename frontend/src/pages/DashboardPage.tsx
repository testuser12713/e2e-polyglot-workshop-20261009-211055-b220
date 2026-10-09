import { useEffect, useState } from 'react'
import { ApiError, apiFetch } from '../api/client'
import type { Dashboard } from '../api/types'
import Tabs from '../components/Tabs'
import type { TabItem } from '../components/Tabs'

const WORKSHOP_TABS: TabItem[] = [
  { to: '/werkstatt/auftraege', label: 'Aufträge' },
  { to: '/werkstatt/dashboard', label: 'Dashboard' },
]

const NON_BREAKING_SPACE = '\u00A0'

const DASHBOARD_STYLES = `
.dashboard__grid { display: grid; grid-template-columns: minmax(0, 1fr); gap: var(--space-3); }
.dashboard__stat-label { font-size: 14px; line-height: 20px; color: var(--color-muted); }
.dashboard__stat-value { margin-top: var(--space-1); font-size: 30px; line-height: 38px; font-weight: 600; font-variant-numeric: tabular-nums; }
.dashboard__stat-money { font-family: var(--font-mono); }
@media (min-width: 640px) {
  .dashboard__grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}
@media (min-width: 1024px) {
  .dashboard__grid { grid-template-columns: repeat(3, minmax(0, 1fr)); gap: var(--space-4); }
}
`

function formatMoney(cents: number): string {
  const value = new Intl.NumberFormat('de-DE', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(cents / 100)
  return `${value}${NON_BREAKING_SPACE}€`
}

function errorMessage(error: unknown): string {
  if (error instanceof ApiError) {
    return error.message
  }
  return 'Ein unerwarteter Fehler ist aufgetreten.'
}

export default function DashboardPage() {
  const [dashboard, setDashboard] = useState<Dashboard | null>(null)
  const [loading, setLoading] = useState(true)
  const [loadError, setLoadError] = useState<string | null>(null)

  useEffect(() => {
    let active = true
    setLoading(true)
    setLoadError(null)
    apiFetch<Dashboard>('/workshop/dashboard')
      .then((data) => {
        if (active) {
          setDashboard(data)
        }
      })
      .catch((error: unknown) => {
        if (active) {
          setLoadError(errorMessage(error))
        }
      })
      .finally(() => {
        if (active) {
          setLoading(false)
        }
      })
    return () => {
      active = false
    }
  }, [])

  return (
    <section className="page-section">
      <style>{DASHBOARD_STYLES}</style>
      <h1 className="page-title">Dashboard</h1>

      <Tabs items={WORKSHOP_TABS} />

      {loading ? (
        <p className="muted" role="status">
          Dashboard wird geladen …
        </p>
      ) : null}

      {!loading && loadError ? (
        <div className="alert alert--danger" role="alert">
          <span>{loadError}</span>
        </div>
      ) : null}

      {!loading && !loadError && dashboard ? (
        <div className="dashboard__grid">
          <div className="card">
            <div className="dashboard__stat-label">Offene Aufträge</div>
            <div className="dashboard__stat-value">{dashboard.open_orders}</div>
          </div>
          <div className="card">
            <div className="dashboard__stat-label">Heute fertig geworden</div>
            <div className="dashboard__stat-value">{dashboard.finished_today}</div>
          </div>
          <div className="card">
            <div className="dashboard__stat-label">Umsatz laufender Monat</div>
            <div className="dashboard__stat-value dashboard__stat-money">
              {formatMoney(dashboard.revenue_month_cents)}
            </div>
          </div>
        </div>
      ) : null}
    </section>
  )
}
