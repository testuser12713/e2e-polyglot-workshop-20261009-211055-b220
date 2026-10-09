import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render, screen } from '@testing-library/react'
import DashboardPage from './DashboardPage'
import { ApiError, apiFetch } from '../api/client'
import type { Dashboard } from '../api/types'

vi.mock('../api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../api/client')>()
  return { ...actual, apiFetch: vi.fn() }
})

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

    render(<DashboardPage />)

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

    render(<DashboardPage />)

    expect(await screen.findByText(/0,05/)).toBeTruthy()
  })

  it('shows a German error message when loading fails', async () => {
    mockedApiFetch.mockRejectedValue(
      new ApiError(500, 'server_error', 'Serverfehler beim Laden des Dashboards.'),
    )

    render(<DashboardPage />)

    const alert = await screen.findByRole('alert')
    expect(alert.textContent).toContain('Serverfehler beim Laden des Dashboards.')
    expect(screen.queryByText('Offene Aufträge')).toBeNull()
  })
})
