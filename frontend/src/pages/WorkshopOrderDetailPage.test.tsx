import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import WorkshopOrderDetailPage from './WorkshopOrderDetailPage'
import { ApiError, apiFetch } from '../api/client'
import type { OrderStatus, WorkshopOrderDetail } from '../api/types'

vi.mock('../api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../api/client')>()
  return { ...actual, apiFetch: vi.fn() }
})

const mockedApiFetch = vi.mocked(apiFetch)

const ORDER_NUMBER = 'AU-2026-0042'

function makeDetail(overrides: Partial<WorkshopOrderDetail> = {}): WorkshopOrderDetail {
  return {
    order: {
      order_number: ORDER_NUMBER,
      status: 'angefragt',
      desired_date: '2026-06-04',
      problem: 'Bremsen quietschen beim Anfahren.',
    },
    customer: {
      id: 1,
      name: 'Erika Muster',
      email: 'erika@example.org',
      phone: '030 1234567',
    },
    vehicle: {
      plate: 'B-AB 1234',
      make: 'VW',
      model: 'Golf',
      mileage: 123456,
    },
    positions: {
      labor_minutes: 90,
      parts: [{ description: 'Ölwechsel', quantity: 1, unit_price_cents: 5000 }],
    },
    history: [
      { status: 'angefragt', changed_at: '2026-06-01T08:00:00Z' },
      { status: 'bestätigt', changed_at: '2026-06-02T09:30:00Z' },
    ],
    invoice: null,
    ...overrides,
  }
}

function renderPage(path = `/werkstatt/auftraege/${ORDER_NUMBER}`) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/werkstatt/auftraege/:number" element={<WorkshopOrderDetailPage />} />
      </Routes>
    </MemoryRouter>,
  )
}

beforeEach(() => {
  mockedApiFetch.mockReset()
})

afterEach(() => {
  cleanup()
})

describe('WorkshopOrderDetailPage', () => {
  it('confirms an order by setting the single legal next status', async () => {
    const detail = makeDetail()
    const history = [
      { status: 'angefragt' as OrderStatus, changed_at: '2026-06-01T08:00:00Z' },
      { status: 'bestätigt' as OrderStatus, changed_at: '2026-06-03T10:00:00Z' },
    ]
    mockedApiFetch.mockImplementation((path, init) => {
      if (init?.method === 'POST') {
        return Promise.resolve({ status: 'bestätigt', history })
      }
      expect(path).toBe(`/workshop/orders/${ORDER_NUMBER}`)
      return Promise.resolve(detail)
    })

    renderPage()

    const confirm = await screen.findByRole('button', { name: 'Auf „bestätigt“ setzen' })
    fireEvent.click(confirm)

    await waitFor(() =>
      expect(mockedApiFetch).toHaveBeenCalledWith(
        `/workshop/orders/${ORDER_NUMBER}/status`,
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({ status: 'bestätigt' }),
        }),
      ),
    )

    expect(await screen.findByRole('button', { name: 'Auf „in Arbeit“ setzen' })).toBeTruthy()
  })

  it('saves labor minutes and parts through the positions endpoint', async () => {
    const detail = makeDetail({
      order: { ...makeDetail().order, status: 'bestätigt' },
    })
    mockedApiFetch.mockResolvedValueOnce(detail).mockResolvedValueOnce({})

    renderPage()

    const labor = await screen.findByLabelText('Arbeitszeit')
    fireEvent.change(labor, { target: { value: '120' } })

    const quantity = screen.getByLabelText('Menge')
    fireEvent.change(quantity, { target: { value: '2' } })

    fireEvent.click(screen.getByRole('button', { name: 'Positionen speichern' }))

    await waitFor(() =>
      expect(mockedApiFetch).toHaveBeenCalledWith(
        `/workshop/orders/${ORDER_NUMBER}/positions`,
        expect.objectContaining({
          method: 'PUT',
          body: JSON.stringify({
            labor_minutes: 120,
            parts: [{ description: 'Ölwechsel', quantity: 2, unit_price_cents: 5000 }],
          }),
        }),
      ),
    )
  })

  it('offers only the one legal next status and no control on the terminal status', async () => {
    mockedApiFetch.mockResolvedValue(makeDetail())
    const { unmount } = renderPage()

    await screen.findByRole('button', { name: 'Auf „bestätigt“ setzen' })
    expect(screen.queryByRole('button', { name: 'Auf „in Arbeit“ setzen' })).toBeNull()
    expect(screen.queryByRole('button', { name: 'Auf „fertig“ setzen' })).toBeNull()
    expect(screen.queryByRole('button', { name: 'Auf „abgeholt“ setzen' })).toBeNull()

    unmount()
    mockedApiFetch.mockReset()
    mockedApiFetch.mockResolvedValue(
      makeDetail({ order: { ...makeDetail().order, status: 'abgeholt' } }),
    )
    renderPage()

    await screen.findByText('abgeholt', { selector: '.status-badge' })
    expect(screen.queryByRole('button', { name: /^Auf „/ })).toBeNull()
  })

  it('shows a German message when the order does not exist', async () => {
    mockedApiFetch.mockRejectedValue(new ApiError(404, 'not_found', 'Auftrag nicht gefunden'))

    renderPage('/werkstatt/auftraege/AU-2026-9999')

    expect(await screen.findByText(/wurde nicht gefunden/)).toBeTruthy()
    expect(screen.getByRole('link', { name: 'Zur Auftragsliste' })).toBeTruthy()
  })

  it('shows an input hint only after the field has been edited', async () => {
    mockedApiFetch.mockResolvedValue(
      makeDetail({ positions: { labor_minutes: 0, parts: [] } }),
    )

    renderPage()

    fireEvent.click(await screen.findByRole('button', { name: 'Position hinzufügen' }))

    expect(screen.queryByText('Menge: ganze Zahl ab 1.')).toBeNull()

    fireEvent.change(screen.getByLabelText('Menge'), { target: { value: '0' } })

    expect(await screen.findByText('Menge: ganze Zahl ab 1.')).toBeTruthy()
  })
})
