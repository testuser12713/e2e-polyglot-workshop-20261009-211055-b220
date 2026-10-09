import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import DashboardPage from './DashboardPage'
import { ApiError, apiFetch } from '../api/client'
import type { Dashboard } from '../api/types'

vi.mock('../api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../api/client')>()
  return { ...actual, apiFetch: vi.fn() }
})

function renderPage() {
  return render(
    <MemoryRouter initialEntries={['/werkstatt/dashboard']}>
      <Routes>
        <Route path="/werkstatt/dashboard" element={<DashboardPage />} />
        <Route path="/werkstatt/auftraege" element={<p>Auftragsseite</p>} />
      </Routes>
    </MemoryRouter>,
  )
}

const mockedApiFetch = vi.mocked(apiFetch)

const DASHBOARD: Dashboard = {
  open_orders: 7,
  finished_today: 3,
  revenue_month_cents: 1234567,
}

beforeEach(() => {
  mockedApiFetch.mockReset()
})

afterEach(() => {
  cleanup()
})

describe('DashboardPage', () => {
  it('loads the dashboard from the workshop endpoint and renders the three figures', async () => {
    mockedApiFetch.mockResolvedValue(DASHBOARD)

    renderPage()

    expect(await screen.findByText('7')).toBeTruthy()
    expect(screen.getByText('3')).toBeTruthy()
    expect(screen.getByText(/12\.345,67/)).toBeTruthy()

    expect(screen.getByText('Offene Aufträge')).toBeTruthy()
    expect(screen.getByText('Heute fertig geworden')).toBeTruthy()
    expect(screen.getByText('Umsatz laufender Monat')).toBeTruthy()

    expect(mockedApiFetch).toHaveBeenCalledWith('/workshop/dashboard')
  })

  it('formats the monthly revenue as euro from whole cents', async () => {
    mockedApiFetch.mockResolvedValue({ ...DASHBOARD, revenue_month_cents: 5 })

    renderPage()

    expect(await screen.findByText(/0,05/)).toBeTruthy()
  })

  it('shows a German error message when loading fails', async () => {
    mockedApiFetch.mockRejectedValue(
      new ApiError(500, 'server_error', 'Serverfehler beim Laden des Dashboards.'),
    )

    renderPage()

    const alert = await screen.findByRole('alert')
    expect(alert.textContent).toContain('Serverfehler beim Laden des Dashboards.')
    expect(screen.queryByText('Offene Aufträge')).toBeNull()
  })

  it('renders both workshop tabs with the current page marked active', async () => {
    mockedApiFetch.mockResolvedValue(DASHBOARD)
    renderPage()

    await screen.findByText('7')

    const ordersTab = screen.getByRole('link', { name: 'Aufträge' })
    const dashboardTab = screen.getByRole('link', { name: 'Dashboard' })

    expect(dashboardTab.getAttribute('aria-current')).toBe('page')
    expect(ordersTab.getAttribute('aria-current')).toBeNull()
  })

  it('navigates to the sibling orders route when the Aufträge tab is clicked', async () => {
    mockedApiFetch.mockResolvedValue(DASHBOARD)
    renderPage()

    await screen.findByText('7')

    fireEvent.click(screen.getByRole('link', { name: 'Aufträge' }))

    expect(await screen.findByText('Auftragsseite')).toBeTruthy()
  })
})
