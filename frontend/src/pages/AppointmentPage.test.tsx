import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { addDays, format } from 'date-fns'
import { de } from 'date-fns/locale'
import AppointmentPage from './AppointmentPage'

function renderPage() {
  return render(
    <MemoryRouter initialEntries={['/auftrag']}>
      <AppointmentPage />
    </MemoryRouter>,
  )
}

vi.mock('../api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../api/client')>()
  return { ...actual, apiFetch: vi.fn() }
})

import { ApiError, apiFetch } from '../api/client'

const mockedApiFetch = vi.mocked(apiFetch)

function fillValidForm() {
  fireEvent.change(screen.getByLabelText('Name'), { target: { value: 'Anna Beispiel' } })
  fireEvent.change(screen.getByLabelText('E-Mail'), { target: { value: 'anna@example.com' } })
  fireEvent.change(screen.getByLabelText('Telefon'), { target: { value: '030 1234567' } })
  fireEvent.change(screen.getByLabelText('Kennzeichen'), { target: { value: 'B-AB 1234' } })
  fireEvent.change(screen.getByLabelText('Marke'), { target: { value: 'VW' } })
  fireEvent.change(screen.getByLabelText('Modell'), { target: { value: 'Golf' } })
  fireEvent.change(screen.getByLabelText('Kilometerstand (km)'), { target: { value: '12345' } })
  fireEvent.change(screen.getByLabelText('Problembeschreibung'), {
    target: { value: 'Die Bremsen quietschen beim Anhalten.' },
  })
}

function pickFutureDate(): Date {
  fireEvent.click(screen.getByLabelText('Wunschtermin'))
  const target = addDays(new Date(), 1)
  const label = format(target, 'PPPP', { locale: de })
  fireEvent.click(screen.getByRole('button', { name: label }))
  return target
}

beforeEach(() => {
  mockedApiFetch.mockReset()
})

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
})

describe('AppointmentPage', () => {
  it('keeps the untouched form neutral and reveals validation only after submit', () => {
    renderPage()

    expect(screen.queryByText('Bitte geben Sie Ihren Namen ein.')).toBeNull()
    expect(screen.queryByRole('alert')).toBeNull()

    fireEvent.click(screen.getByRole('button', { name: 'Termin anfragen' }))

    expect(screen.getByText('Bitte geben Sie Ihren Namen ein.')).toBeTruthy()
    expect(screen.getByText('Bitte wählen Sie einen Wunschtermin aus.')).toBeTruthy()
    expect(mockedApiFetch).not.toHaveBeenCalled()
  })

  it('shows a field error only after the field has been touched', () => {
    renderPage()
    const name = screen.getByLabelText('Name')

    fireEvent.change(name, { target: { value: 'Anna' } })
    fireEvent.blur(name)
    expect(screen.queryByText('Bitte geben Sie Ihren Namen ein.')).toBeNull()

    fireEvent.change(name, { target: { value: '' } })
    fireEvent.blur(name)
    expect(screen.getByText('Bitte geben Sie Ihren Namen ein.')).toBeTruthy()
  })

  it('submits the request and shows the returned order number prominently', async () => {
    mockedApiFetch.mockResolvedValue({ order_number: 'AU-2026-0042', status: 'angefragt' })

    renderPage()
    fillValidForm()
    const target = pickFutureDate()
    fireEvent.click(screen.getByRole('button', { name: 'Termin anfragen' }))

    expect(await screen.findByText('AU-2026-0042')).toBeTruthy()
    expect(mockedApiFetch).toHaveBeenCalledTimes(1)

    const [path, init] = mockedApiFetch.mock.calls[0]
    expect(path).toBe('/appointments')
    expect(init?.method).toBe('POST')
    const body = JSON.parse(String(init?.body)) as {
      customer: { name: string; email: string; phone: string }
      vehicle: { plate: string; make: string; model: string; mileage: number }
      desired_date: string
      problem: string
    }
    expect(body.customer).toEqual({
      name: 'Anna Beispiel',
      email: 'anna@example.com',
      phone: '030 1234567',
    })
    expect(body.vehicle).toEqual({
      plate: 'B-AB 1234',
      make: 'VW',
      model: 'Golf',
      mileage: 12345,
    })
    expect(body.desired_date).toBe(format(target, 'yyyy-MM-dd'))
    expect(body.problem).toBe('Die Bremsen quietschen beim Anhalten.')
  })

  it('accepts a Wunschtermin typed as TT.MM.JJJJ without opening the calendar', async () => {
    mockedApiFetch.mockResolvedValue({ order_number: 'AU-2026-0042', status: 'angefragt' })

    renderPage()
    fillValidForm()
    const target = addDays(new Date(), 3)
    const input = screen.getByLabelText('Wunschtermin')
    fireEvent.change(input, { target: { value: format(target, 'dd.MM.yyyy') } })
    fireEvent.blur(input)
    fireEvent.click(screen.getByRole('button', { name: 'Termin anfragen' }))

    expect(await screen.findByText('AU-2026-0042')).toBeTruthy()
    expect(mockedApiFetch).toHaveBeenCalledTimes(1)
    const [, init] = mockedApiFetch.mock.calls[0]
    expect(JSON.parse(String(init?.body)).desired_date).toBe(format(target, 'yyyy-MM-dd'))
  })

  it('renders the returned order number inside a polite status region', async () => {
    mockedApiFetch.mockResolvedValue({ order_number: 'AU-2026-0099', status: 'angefragt' })

    renderPage()
    fillValidForm()
    pickFutureDate()
    fireEvent.click(screen.getByRole('button', { name: 'Termin anfragen' }))

    const status = await screen.findByRole('status')
    expect(status.getAttribute('aria-live')).toBe('polite')
    expect(status.textContent).toContain('AU-2026-0099')
  })

  it('renders an API error as a German message next to the form', async () => {
    mockedApiFetch.mockRejectedValue(
      new ApiError(400, 'invalid_email', 'Die E-Mail-Adresse ist ungültig.'),
    )

    renderPage()
    fillValidForm()
    pickFutureDate()
    fireEvent.click(screen.getByRole('button', { name: 'Termin anfragen' }))

    expect(await screen.findByText('Die E-Mail-Adresse ist ungültig.')).toBeTruthy()
    expect(screen.getByRole('button', { name: 'Termin anfragen' })).toBeTruthy()
  })

  it('renders a network failure as a German message', async () => {
    mockedApiFetch.mockRejectedValue(
      new ApiError(0, 'network_error', 'Die Verbindung zum Server ist fehlgeschlagen.'),
    )

    renderPage()
    fillValidForm()
    pickFutureDate()
    fireEvent.click(screen.getByRole('button', { name: 'Termin anfragen' }))

    expect(await screen.findByText('Die Verbindung zum Server ist fehlgeschlagen.')).toBeTruthy()
  })
})
