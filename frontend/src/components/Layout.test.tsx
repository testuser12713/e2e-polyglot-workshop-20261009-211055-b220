import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, within } from '@testing-library/react'
import App from '../App'
import { apiFetch } from '../api/client'

function navigate(path: string) {
  window.history.pushState({}, '', path)
}

beforeEach(() => {
  window.localStorage.clear()
})

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
  window.history.pushState({}, '', '/')
})

describe('shell', () => {
  it('renders the navigation of both areas and the legal footer links', () => {
    navigate('/')
    render(<App />)

    const header = screen.getByRole('banner')
    expect(within(header).getAllByRole('link', { name: 'Kundenbereich' }).length).toBeGreaterThan(0)
    expect(within(header).getAllByRole('link', { name: 'Werkstattbereich' }).length).toBeGreaterThan(
      0,
    )

    expect(screen.getByRole('navigation', { name: 'Rechtliches' })).toBeTruthy()
    expect(screen.getByRole('link', { name: 'Impressum' })).toBeTruthy()
    expect(screen.getByRole('link', { name: 'Datenschutzerklärung' })).toBeTruthy()
  })

  it('opens the Impressum page behind the shared shell', () => {
    navigate('/impressum')
    render(<App />)

    expect(screen.getByRole('heading', { level: 1, name: 'Impressum' })).toBeTruthy()
    expect(screen.getByText(/Angaben gemäß § 5 DDG/)).toBeTruthy()
    expect(screen.getByRole('link', { name: 'Datenschutzerklärung' })).toBeTruthy()
  })

  it('opens the Datenschutz page behind the shared shell', () => {
    navigate('/datenschutz')
    render(<App />)

    expect(screen.getByRole('heading', { level: 1, name: 'Datenschutzerklärung' })).toBeTruthy()
    expect(screen.getByText(/Keine externen Ressourcen/)).toBeTruthy()
    expect(screen.getByRole('link', { name: 'Impressum' })).toBeTruthy()
  })

  it('sends an unauthenticated visitor from the workshop area to the login route', () => {
    navigate('/werkstatt/dashboard')
    render(<App />)

    expect(
      screen.getByRole('heading', { level: 1, name: 'Anmeldung Werkstattbereich' }),
    ).toBeTruthy()
  })

  it('loads no resource from a third-party host', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      statusText: 'OK',
      headers: new Headers({ 'content-type': 'application/json' }),
      json: async () => ({ status: 'ok', database: 'ok', queue: 'ok' }),
    })
    vi.stubGlobal('fetch', fetchMock)

    await apiFetch('/health')

    expect(fetchMock).toHaveBeenCalledTimes(1)
    const calledUrl = new URL(String(fetchMock.mock.calls[0][0]))
    expect(['localhost', '127.0.0.1']).toContain(calledUrl.hostname)

    navigate('/')
    const { container } = render(<App />)
    container.querySelectorAll<HTMLElement>('[src], [href]').forEach((element) => {
      const value = element.getAttribute('src') ?? element.getAttribute('href') ?? ''
      if (/^(https?:)?\/\//.test(value)) {
        const url = new URL(value, window.location.origin)
        expect(url.origin).toBe(window.location.origin)
      }
    })
  })
})

describe('top navigation', () => {
  it('keeps only the two area links and no sub-page links in the header', () => {
    navigate('/')
    render(<App />)

    const header = screen.getByRole('banner')
    const links = within(header).getAllByRole('link')

    expect(within(header).getAllByRole('link', { name: 'Kundenbereich' }).length).toBeGreaterThan(0)
    expect(within(header).getAllByRole('link', { name: 'Werkstattbereich' }).length).toBeGreaterThan(
      0,
    )

    const labels = links.map((link) => link.textContent?.trim())
    expect(labels).not.toContain('Status abrufen')
    expect(labels).not.toContain('Termin anfragen')
    expect(labels).not.toContain('Aufträge')
    expect(labels).not.toContain('Dashboard')
  })

  it('shows the brand as plain text without a link role', () => {
    navigate('/')
    render(<App />)

    const header = screen.getByRole('banner')
    expect(within(header).queryByRole('link', { name: /Kfz-Werkstatt/ })).toBeNull()
    expect(within(header).getByText('Kfz-Werkstatt')).toBeTruthy()
  })

  it('navigates from the customer area back to the start through the Kundenbereich entry', () => {
    navigate('/impressum')
    render(<App />)

    const header = screen.getByRole('banner')
    fireEvent.click(within(header).getByRole('link', { name: 'Kundenbereich' }))

    expect(screen.getByRole('heading', { level: 1, name: 'Status abrufen' })).toBeTruthy()
  })

  it('shows the workshop area link without a login', () => {
    navigate('/')
    render(<App />)

    const header = screen.getByRole('banner')
    expect(within(header).getAllByRole('link', { name: 'Werkstattbereich' }).length).toBeGreaterThan(
      0,
    )
  })

  it('shows the employee name and Abmelden after login', () => {
    window.localStorage.setItem('werkstatt.token', 'test-token')
    window.localStorage.setItem(
      'werkstatt.employee',
      JSON.stringify({ email: 'anna.meier@example.com', name: 'Anna Meier' }),
    )
    navigate('/')
    render(<App />)

    const header = screen.getByRole('banner')
    expect(within(header).getByText('Anna Meier')).toBeTruthy()
    expect(within(header).getByRole('button', { name: 'Abmelden' })).toBeTruthy()
  })
})
