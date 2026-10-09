import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom'
import { AuthProvider } from '../auth/AuthContext'
import { ApiError, apiFetch } from '../api/client'
import type { LoginResponse } from '../api/types'
import LoginPage from './LoginPage'

vi.mock('../api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../api/client')>()
  return { ...actual, apiFetch: vi.fn() }
})

const apiFetchMock = vi.mocked(apiFetch)

const LOGIN_RESPONSE: LoginResponse = {
  token: 'test-token',
  employee: { email: 'anna@werkstatt.de', name: 'Anna Meister' },
}

function RouteProbe() {
  const location = useLocation()
  return <h1>Route: {location.pathname}</h1>
}

function renderLogin(from?: string) {
  return render(
    <MemoryRouter
      initialEntries={[{ pathname: '/werkstatt/login', state: from ? { from } : undefined }]}
    >
      <AuthProvider>
        <Routes>
          <Route path="/werkstatt/login" element={<LoginPage />} />
          <Route path="/werkstatt/auftraege" element={<RouteProbe />} />
          <Route path="/werkstatt/dashboard" element={<RouteProbe />} />
        </Routes>
      </AuthProvider>
    </MemoryRouter>,
  )
}

function fillCredentials(email = 'anna@werkstatt.de', password = 'geheim') {
  fireEvent.change(screen.getByLabelText('E-Mail'), { target: { value: email } })
  fireEvent.change(screen.getByLabelText('Passwort'), { target: { value: password } })
}

function submit() {
  fireEvent.click(screen.getByRole('button', { name: /Anmeld/ }))
}

beforeEach(() => {
  window.localStorage.clear()
})

afterEach(() => {
  cleanup()
  vi.clearAllMocks()
})

describe('LoginPage', () => {
  it('shows no field errors on an untouched form', () => {
    apiFetchMock.mockResolvedValue(LOGIN_RESPONSE)
    renderLogin()

    expect(screen.queryByText('Bitte geben Sie Ihre E-Mail-Adresse ein.')).toBeNull()
    expect(screen.queryByText('Bitte geben Sie Ihr Passwort ein.')).toBeNull()
  })

  it('logs in and redirects to the originally requested workshop route', async () => {
    apiFetchMock.mockResolvedValue(LOGIN_RESPONSE)
    renderLogin('/werkstatt/dashboard')

    fillCredentials()
    submit()

    await waitFor(() => {
      expect(screen.getByText('Route: /werkstatt/dashboard')).toBeTruthy()
    })
    expect(apiFetchMock).toHaveBeenCalledWith(
      '/auth/login',
      expect.objectContaining({ method: 'POST' }),
    )
  })

  it('falls back to the order list when no origin route was remembered', async () => {
    apiFetchMock.mockResolvedValue(LOGIN_RESPONSE)
    renderLogin()

    fillCredentials()
    submit()

    await waitFor(() => {
      expect(screen.getByText('Route: /werkstatt/auftraege')).toBeTruthy()
    })
  })

  it('shows the German message from the API error body on wrong credentials', async () => {
    apiFetchMock.mockRejectedValue(
      new ApiError(401, 'invalid_credentials', 'E-Mail-Adresse oder Passwort ist falsch.'),
    )
    renderLogin()

    fillCredentials('anna@werkstatt.de', 'falsch')
    submit()

    await waitFor(() => {
      expect(screen.getByRole('alert').textContent).toContain(
        'E-Mail-Adresse oder Passwort ist falsch.',
      )
    })
  })

  it('shows a friendly wait hint and clears the password on a 429 answer', async () => {
    apiFetchMock.mockRejectedValue(new ApiError(429, 'rate_limited', 'Zu viele Versuche.'))
    renderLogin()

    fillCredentials('anna@werkstatt.de', 'geheim')
    submit()

    await waitFor(() => {
      expect(screen.getByRole('alert').textContent).toContain('Bitte warten Sie eine Minute')
    })
    expect((screen.getByLabelText('Passwort') as HTMLInputElement).value).toBe('')
  })

  it('disables the submit button and both fields while the request runs', async () => {
    let resolveLogin: (value: LoginResponse) => void = () => {}
    apiFetchMock.mockReturnValue(
      new Promise<LoginResponse>((resolve) => {
        resolveLogin = resolve
      }),
    )
    renderLogin()

    fillCredentials()
    submit()

    const submitButton = screen.getByRole('button', { name: /Anmeld/ }) as HTMLButtonElement
    expect(submitButton.disabled).toBe(true)
    expect((screen.getByLabelText('E-Mail') as HTMLInputElement).disabled).toBe(true)
    expect((screen.getByLabelText('Passwort') as HTMLInputElement).disabled).toBe(true)

    resolveLogin(LOGIN_RESPONSE)
    await waitFor(() => {
      expect(screen.getByText('Route: /werkstatt/auftraege')).toBeTruthy()
    })
  })

  it('reveals the password when the reveal control is used', () => {
    apiFetchMock.mockResolvedValue(LOGIN_RESPONSE)
    renderLogin()

    const password = screen.getByLabelText('Passwort') as HTMLInputElement
    expect(password.type).toBe('password')

    fireEvent.click(screen.getByRole('button', { name: 'Passwort anzeigen' }))
    expect((screen.getByLabelText('Passwort') as HTMLInputElement).type).toBe('text')
  })
})
