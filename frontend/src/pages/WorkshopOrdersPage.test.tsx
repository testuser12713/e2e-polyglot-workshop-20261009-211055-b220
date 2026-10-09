import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import WorkshopOrdersPage from './WorkshopOrdersPage'
import { ApiError, apiFetch } from '../api/client'
import type { WorkshopOrderList } from '../api/types'

vi.mock('../api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../api/client')>()
  return { ...actual, apiFetch: vi.fn() }
})

const mockedFetch = vi.mocked(apiFetch)

const ORDERS: WorkshopOrderList = {
  orders: [
    {
      order_number: 'AU-2026-0042',
      status: 'in Arbeit',
      plate: 'B-AB 1234',
      customer_name: 'Anna Muster',
    },
    {
      order_number: 'AU-2026-0043',
      status: 'fertig',
      plate: 'M-CX 9876',
      customer_name: 'Bernd Beispiel',
    },
  ],
}

function lastPath(): string {
  const calls = mockedFetch.mock.calls
  return String(calls[calls.length - 1]?.[0] ?? '')
}

function renderPage() {
  return render(
    <MemoryRouter initialEntries={['/werkstatt/auftraege']}>
      <Routes>
        <Route path="/werkstatt/auftraege" element={<WorkshopOrdersPage />} />
        <Route
          path="/werkstatt/auftraege/:number"
          element={<p>Detailseite Auftrag</p>}
        />
        <Route path="/werkstatt/dashboard" element={<p>Dashboardseite</p>} />
      </Routes>
    </MemoryRouter>,
  )
}

beforeEach(() => {
  mockedFetch.mockReset()
  mockedFetch.mockResolvedValue(ORDERS)
})

afterEach(() => {
  cleanup()
})

describe('WorkshopOrdersPage', () => {
  it('loads and renders the orders with number, status, plate and customer', async () => {
    renderPage()

    expect(await screen.findByText('AU-2026-0042')).toBeTruthy()
    expect(screen.getByText('B-AB 1234')).toBeTruthy()
    expect(screen.getByText('Anna Muster')).toBeTruthy()
    expect(screen.getAllByText('in Arbeit').length).toBeGreaterThan(0)
    expect(mockedFetch).toHaveBeenCalledWith('/workshop/orders')
  })

  it('passes the chosen status to the API when filtering', async () => {
    renderPage()
    await screen.findByText('AU-2026-0042')

    fireEvent.change(screen.getByLabelText('Status'), { target: { value: 'fertig' } })

    await waitFor(() => {
      expect(lastPath()).toContain('status=fertig')
    })
    expect(lastPath()).toBe('/workshop/orders?status=fertig')
  })

  it('passes the uppercased plate to the API when searching', async () => {
    renderPage()
    await screen.findByText('AU-2026-0042')

    const plateField = screen.getByLabelText('Kennzeichen')
    fireEvent.change(plateField, { target: { value: 'b-ab 1234' } })
    expect((plateField as HTMLInputElement).value).toBe('B-AB 1234')

    fireEvent.click(screen.getByRole('button', { name: 'Suchen' }))

    await waitFor(() => {
      expect(lastPath()).toContain('plate=B-AB')
    })
    expect(decodeURIComponent(lastPath().replace(/\+/g, ' '))).toContain('plate=B-AB 1234')
  })

  it('navigates to the detail route when a row is clicked', async () => {
    renderPage()
    await screen.findByText('AU-2026-0042')

    fireEvent.click(screen.getByText('Anna Muster'))

    expect(await screen.findByText('Detailseite Auftrag')).toBeTruthy()
  })

  it('renders both workshop tabs with the current page marked active', async () => {
    renderPage()
    await screen.findByText('AU-2026-0042')

    const ordersTab = screen.getByRole('link', { name: 'Aufträge' })
    const dashboardTab = screen.getByRole('link', { name: 'Dashboard' })

    expect(ordersTab.getAttribute('aria-current')).toBe('page')
    expect(dashboardTab.getAttribute('aria-current')).toBeNull()
  })

  it('navigates to the sibling dashboard route when the Dashboard tab is clicked', async () => {
    renderPage()
    await screen.findByText('AU-2026-0042')

    fireEvent.click(screen.getByRole('link', { name: 'Dashboard' }))

    expect(await screen.findByText('Dashboardseite')).toBeTruthy()
  })

  it('shows a German empty-state message when no order matches', async () => {
    mockedFetch.mockResolvedValue({ orders: [] })
    renderPage()

    expect(await screen.findByText('Keine Aufträge gefunden')).toBeTruthy()
  })

  it('shows a German error message when the API fails', async () => {
    mockedFetch.mockRejectedValue(new ApiError(500, 'server_error', 'Unerwarteter Fehler'))
    renderPage()

    const alert = await screen.findByRole('alert')
    expect(alert.textContent).toContain('server_error')
  })
})
