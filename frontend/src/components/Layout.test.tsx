import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render, screen } from '@testing-library/react'
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

    expect(screen.getAllByRole('link', { name: 'Kundenbereich' }).length).toBeGreaterThan(0)
    expect(screen.getAllByRole('link', { name: 'Werkstattbereich' }).length).toBeGreaterThan(0)

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
