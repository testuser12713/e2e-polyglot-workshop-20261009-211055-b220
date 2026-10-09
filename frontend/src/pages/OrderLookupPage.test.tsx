import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import OrderLookupPage from './OrderLookupPage'
import { ApiError, apiFetch } from '../api/client'
import type { Invoice, PublicOrderStatus } from '../api/types'

vi.mock('../api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../api/client')>()
  return { ...actual, apiFetch: vi.fn() }
})

const mockedApiFetch = vi.mocked(apiFetch)

const ORDER_NUMBER = 'AU-2026-0042'
const PLATE = 'B-AB 1234'

const STATUS: PublicOrderStatus = {
  order_number: ORDER_NUMBER,
  status: 'fertig',
  vehicle: { plate: PLATE, make: 'VW', model: 'Golf', mileage: 123456 },
  problem: 'Bremsen quietschen beim Anfahren.',
  history: [
    { status: 'angefragt', changed_at: '2026-06-01T08:00:00Z' },
    { status: 'bestätigt', changed_at: '2026-06-02T09:30:00Z' },
    { status: 'in Arbeit', changed_at: '2026-06-03T10:00:00Z' },
    { status: 'fertig', changed_at: '2026-06-04T11:00:00Z' },
  ],
}

const INVOICE: Invoice = {
  invoice_number: 'RE-2026-0001',
  issued_at: '2026-06-04T12:00:00Z',
  labor_minutes: 90,
  items: [{ description: 'Ölwechsel', quantity: 2, unit_price_cents: 5000 }],
  net_cents: 10000,
  vat_cents: 1900,
  gross_cents: 11900,
}

function renderPage() {
  return render(
    <MemoryRouter initialEntries={['/']}>
      <OrderLookupPage />
    </MemoryRouter>,
  )
}

function fillAndSubmit() {
  fireEvent.change(screen.getByLabelText('Auftragsnummer'), {
    target: { value: ORDER_NUMBER },
  })
  fireEvent.change(screen.getByLabelText('Kennzeichen'), { target: { value: PLATE } })
  fireEvent.click(screen.getByRole('button', { name: 'Status abrufen' }))
}

beforeEach(() => {
  mockedApiFetch.mockReset()
})

afterEach(() => {
  cleanup()
})

describe('OrderLookupPage', () => {
  it('shows status, vehicle data, history and the invoice for a known combination', async () => {
    mockedApiFetch.mockImplementation((path) => {
      const target = String(path)
      if (target.includes('/invoice')) {
        return Promise.resolve(INVOICE)
      }
      expect(target).toBe(`/orders/${ORDER_NUMBER}/status?plate=B-AB+1234`)
      return Promise.resolve(STATUS)
    })

    renderPage()
    fillAndSubmit()

    expect(await screen.findByText('RE-2026-0001')).toBeTruthy()
    expect(mockedApiFetch).toHaveBeenCalledWith(`/orders/${ORDER_NUMBER}/invoice?plate=B-AB+1234`)

    expect(screen.getByText('VW Golf')).toBeTruthy()
    expect(screen.getByText(PLATE)).toBeTruthy()
    expect(screen.getByText(/123\.456/)).toBeTruthy()
    expect(screen.getByText('Bremsen quietschen beim Anfahren.')).toBeTruthy()

    expect(screen.getByText('fertig', { selector: '[aria-current="step"]' })).toBeTruthy()
    expect(screen.queryByText('angefragt', { selector: '[aria-disabled="true"]' })).toBeNull()
    expect(screen.getByText('abgeholt', { selector: '[aria-disabled="true"]' })).toBeTruthy()

    expect(screen.getByText('Ölwechsel')).toBeTruthy()
    expect(screen.getByText('Netto')).toBeTruthy()
    expect(screen.getByText(/Mehrwertsteuer 19/)).toBeTruthy()
    expect(screen.getByText('Brutto')).toBeTruthy()
    expect(screen.getByText(/119,00\s*€/)).toBeTruthy()
    expect(screen.getByText('04.06.2026, 13:00 Uhr')).toBeTruthy()
  })

  it('shows a German hint above the fields when no invoice exists yet', async () => {
    mockedApiFetch.mockImplementation((path) => {
      if (String(path).includes('/invoice')) {
        return Promise.reject(new ApiError(404, 'not_found', 'Keine Rechnung vorhanden'))
      }
      return Promise.resolve({ ...STATUS, status: 'in Arbeit' })
    })

    renderPage()
    fillAndSubmit()

    const hint = await screen.findByText(/Rechnung noch nicht vorhanden/)
    const alert = hint.closest('[role="status"]')
    expect(alert).toBeTruthy()
    expect(alert?.className).toContain('alert--info')
    const infoPosition = alert!.compareDocumentPosition(screen.getByLabelText('Auftragsnummer'))
    expect(infoPosition & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  })

  it('shows the lookup error above the fields as a danger alert', async () => {
    mockedApiFetch.mockRejectedValue(new ApiError(404, 'not_found', 'Auftrag nicht gefunden'))

    renderPage()
    fillAndSubmit()

    const message = await screen.findByText(/wurde kein Auftrag gefunden/)
    const alert = message.closest('[role="alert"]')
    expect(alert).toBeTruthy()
    expect(alert?.className).toContain('alert--danger')
    const position = alert!.compareDocumentPosition(screen.getByLabelText('Auftragsnummer'))
    expect(position & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  })

  it('renders the customer tabs with the current one active', () => {
    renderPage()

    const nav = screen.getByRole('navigation', { name: 'Kundenbereich' })
    const links = within(nav).getAllByRole('link')
    expect(links.map((link) => link.textContent)).toEqual(['Status abrufen', 'Termin anfragen'])

    const currentTab = within(nav).getByRole('link', { name: 'Status abrufen' })
    expect(currentTab.className).toContain('tabs__link--active')

    const otherTab = within(nav).getByRole('link', { name: 'Termin anfragen' })
    expect(otherTab.className).not.toContain('tabs__link--active')
  })

  it('shows an understandable German message for an unknown combination', async () => {
    mockedApiFetch.mockRejectedValue(
      new ApiError(404, 'not_found', 'Auftrag nicht gefunden'),
    )

    renderPage()
    fillAndSubmit()

    expect(await screen.findByText(/wurde kein Auftrag gefunden/)).toBeTruthy()
    expect(screen.queryByText('RE-2026-0001')).toBeNull()
    expect(screen.queryByRole('heading', { name: 'Status' })).toBeNull()
  })

  it('reveals validation hints only after typing or a submit attempt', async () => {
    renderPage()

    expect(screen.queryByText('Bitte geben Sie die Auftragsnummer ein.')).toBeNull()
    expect(screen.queryByText('Bitte geben Sie das Kennzeichen ein.')).toBeNull()

    fireEvent.click(screen.getByRole('button', { name: 'Status abrufen' }))

    expect(await screen.findByText('Bitte geben Sie die Auftragsnummer ein.')).toBeTruthy()
    expect(screen.getByText('Bitte geben Sie das Kennzeichen ein.')).toBeTruthy()
    expect(mockedApiFetch).not.toHaveBeenCalled()
  })

  it('sends the plate uppercased', async () => {
    mockedApiFetch.mockImplementation((path) => {
      if (String(path).includes('/invoice')) {
        return Promise.resolve(INVOICE)
      }
      return Promise.resolve(STATUS)
    })

    renderPage()
    fireEvent.change(screen.getByLabelText('Auftragsnummer'), {
      target: { value: ORDER_NUMBER },
    })
    fireEvent.change(screen.getByLabelText('Kennzeichen'), { target: { value: 'b-ab 1234' } })
    fireEvent.click(screen.getByRole('button', { name: 'Status abrufen' }))

    await waitFor(() =>
      expect(mockedApiFetch).toHaveBeenCalledWith(
        `/orders/${ORDER_NUMBER}/status?plate=B-AB+1234`,
      ),
    )
  })
})
