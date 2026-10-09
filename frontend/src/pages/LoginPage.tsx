import { useState } from 'react'
import type { CSSProperties, FormEvent } from 'react'
import { Navigate, useLocation } from 'react-router-dom'
import { ApiError } from '../api/client'
import { useAuth } from '../auth/AuthContext'

const DEFAULT_REDIRECT = '/werkstatt/auftraege'

const RATE_LIMIT_MESSAGE =
  'Zu viele Anmeldeversuche. Bitte warten Sie eine Minute und versuchen Sie es erneut.'
const FALLBACK_CREDENTIALS_MESSAGE = 'E-Mail-Adresse oder Passwort ist falsch.'
const UNEXPECTED_ERROR_MESSAGE =
  'Die Anmeldung ist fehlgeschlagen. Bitte versuchen Sie es später erneut.'

interface LoginLocationState {
  from?: string
}

const INVALID_BORDER: CSSProperties = { border: '1px solid #B3261E' }
const FOCUS_BORDER: CSSProperties = { border: '2px solid #17509B' }

/**
 * TextField error state from DESIGN.md, applied inline because this page styles
 * itself with inline style objects: an invalid field keeps the danger border,
 * while focus always wins with the 2px accent border (the 2px focus outline
 * comes from the shared `.field__input:focus` rule).
 */
function fieldBorderStyle(invalid: boolean, focused: boolean): CSSProperties {
  if (focused) {
    return FOCUS_BORDER
  }
  return invalid ? INVALID_BORDER : {}
}

/**
 * Workshop login. Authenticates through `useAuth().login`; on success the user
 * lands on the workshop route they originally requested (set by `RequireAuth`),
 * or the order list by default.
 */
export default function LoginPage() {
  const { login, isAuthenticated } = useAuth()
  const location = useLocation()
  const state = location.state as LoginLocationState | null
  const redirectTo = state?.from ?? DEFAULT_REDIRECT

  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [showPassword, setShowPassword] = useState(false)
  const [emailTouched, setEmailTouched] = useState(false)
  const [passwordTouched, setPasswordTouched] = useState(false)
  const [emailFocused, setEmailFocused] = useState(false)
  const [passwordFocused, setPasswordFocused] = useState(false)
  const [submitted, setSubmitted] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const emailMissing = email.trim() === ''
  const passwordMissing = password === ''
  const showEmailError = emailMissing && (emailTouched || submitted)
  const showPasswordError = passwordMissing && (passwordTouched || submitted)

  if (isAuthenticated) {
    return <Navigate to={redirectTo} replace />
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>): Promise<void> {
    event.preventDefault()
    setSubmitted(true)
    if (emailMissing || passwordMissing) {
      return
    }
    setSubmitting(true)
    setError(null)
    try {
      await login(email.trim(), password)
    } catch (caught) {
      if (caught instanceof ApiError && caught.status === 429) {
        setError(RATE_LIMIT_MESSAGE)
        setPassword('')
        setPasswordTouched(false)
        setSubmitted(false)
      } else if (caught instanceof ApiError && caught.message) {
        setError(caught.message)
      } else if (caught instanceof ApiError) {
        setError(FALLBACK_CREDENTIALS_MESSAGE)
      } else {
        setError(UNEXPECTED_ERROR_MESSAGE)
      }
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <section className="page-section page-section--narrow">
      <form
        className="card"
        noValidate
        onSubmit={handleSubmit}
        style={{
          width: '100%',
          maxWidth: '400px',
          marginInline: 'auto',
          padding: 'var(--space-4)',
        }}
      >
        <h1
          style={{
            fontSize: '24px',
            lineHeight: '32px',
            fontWeight: 600,
            marginBottom: 'var(--space-4)',
          }}
        >
          Anmeldung Werkstattbereich
        </h1>

        {error ? (
          <div
            className="alert alert--danger"
            role="alert"
            style={{ marginBottom: 'var(--space-3)' }}
          >
            {error}
          </div>
        ) : null}

        <div className="field">
          <label className="field__label" htmlFor="login-email">
            E-Mail
          </label>
          <input
            id="login-email"
            className="field__input"
            type="email"
            name="email"
            autoComplete="username"
            value={email}
            disabled={submitting}
            aria-invalid={showEmailError}
            aria-describedby={showEmailError ? 'login-email-error' : undefined}
            style={fieldBorderStyle(showEmailError, emailFocused)}
            onChange={(event) => setEmail(event.target.value)}
            onFocus={() => setEmailFocused(true)}
            onBlur={() => {
              setEmailFocused(false)
              setEmailTouched(true)
            }}
          />
          {showEmailError ? (
            <p className="field__error" id="login-email-error" role="alert">
              Bitte geben Sie Ihre E-Mail-Adresse ein.
            </p>
          ) : null}
        </div>

        <div className="field">
          <label className="field__label" htmlFor="login-password">
            Passwort
          </label>
          <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-1)' }}>
            <input
              id="login-password"
              className="field__input"
              type={showPassword ? 'text' : 'password'}
              name="password"
              autoComplete="current-password"
              value={password}
              disabled={submitting}
              aria-invalid={showPasswordError}
              aria-describedby={showPasswordError ? 'login-password-error' : undefined}
              style={{ flex: 1, minWidth: 0, ...fieldBorderStyle(showPasswordError, passwordFocused) }}
              onChange={(event) => setPassword(event.target.value)}
              onFocus={() => setPasswordFocused(true)}
              onBlur={() => {
                setPasswordFocused(false)
                setPasswordTouched(true)
              }}
            />
            <button
              type="button"
              aria-label={showPassword ? 'Passwort verbergen' : 'Passwort anzeigen'}
              aria-pressed={showPassword}
              disabled={submitting}
              onClick={() => setShowPassword((visible) => !visible)}
              style={{
                display: 'inline-flex',
                alignItems: 'center',
                justifyContent: 'center',
                width: '44px',
                height: '44px',
                flexShrink: 0,
                border: '1px solid transparent',
                borderRadius: 'var(--radius-md)',
                background: 'transparent',
                color: 'var(--color-muted)',
                cursor: 'pointer',
              }}
            >
              {showPassword ? (
                <svg
                  width="20"
                  height="20"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  aria-hidden="true"
                >
                  <path d="M3 3l18 18" />
                  <path d="M10.6 10.6a2 2 0 0 0 2.8 2.8" />
                  <path d="M9.9 5.1A10.9 10.9 0 0 1 12 5c5 0 9 4.5 10 7-.4 1-1.4 2.5-2.8 3.8" />
                  <path d="M6.2 6.2C4.3 7.6 2.8 9.6 2 12c1 2.5 5 7 10 7 1.2 0 2.3-.2 3.3-.6" />
                </svg>
              ) : (
                <svg
                  width="20"
                  height="20"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  aria-hidden="true"
                >
                  <path d="M2 12s3.6-7 10-7 10 7 10 7-3.6 7-10 7-10-7-10-7Z" />
                  <circle cx="12" cy="12" r="3" />
                </svg>
              )}
            </button>
          </div>
          {showPasswordError ? (
            <p className="field__error" id="login-password-error" role="alert">
              Bitte geben Sie Ihr Passwort ein.
            </p>
          ) : null}
        </div>

        <button
          type="submit"
          className="btn btn--primary btn--block"
          disabled={submitting}
          aria-busy={submitting}
        >
          {submitting ? 'Anmeldung läuft…' : 'Anmelden'}
        </button>
      </form>
    </section>
  )
}
