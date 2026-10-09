import { useState } from 'react'
import type { FormEvent, ReactNode } from 'react'
import { ApiError, apiFetch } from '../api/client'
import { ORDER_STATUSES } from '../api/types'
import type { HistoryEntry, Invoice, OrderStatus, PublicOrderStatus } from '../api/types'
import Tabs from '../components/Tabs'

const NON_BREAKING_SPACE = '\u00A0'

function formatMoney(cents: number): string {
  const value = new Intl.NumberFormat('de-DE', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(cents / 100)
  return `${value}${NON_BREAKING_SPACE}€`
}

function toDate(value: string): Date | null {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? null : date
}

function formatDate(value: string): string {
  const date = toDate(value)
  if (!date) {
    return value
  }
  return new Intl.DateTimeFormat('de-DE', {
    day: '2-digit',
    month: '2-digit',
    year: 'numeric',
    timeZone: 'Europe/Berlin',
  }).format(date)
}

function formatDateTime(value: string): string {
  const date = toDate(value)
  if (!date) {
    return value
  }
  const day = new Intl.DateTimeFormat('de-DE', {
    day: '2-digit',
    month: '2-digit',
    year: 'numeric',
    timeZone: 'Europe/Berlin',
  }).format(date)
  const time = new Intl.DateTimeFormat('de-DE', {
    hour: '2-digit',
    minute: '2-digit',
    timeZone: 'Europe/Berlin',
  }).format(date)
  return `${day}, ${time} Uhr`
}

function formatHours(minutes: number): string {
  const hours = new Intl.NumberFormat('de-DE', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(minutes / 60)
  return `${hours} h`
}

function formatMileage(kilometers: number): string {
  const value = new Intl.NumberFormat('de-DE', { maximumFractionDigits: 0 }).format(kilometers)
  return `${value}${NON_BREAKING_SPACE}km`
}

const STATUS_TOKENS: Record<OrderStatus, { fg: string; soft: string }> = {
  angefragt: { fg: 'var(--color-statusAng)', soft: 'var(--color-statusAngSoft)' },
  bestätigt: { fg: 'var(--color-statusBestaetigt)', soft: 'var(--color-statusBestaetigtSoft)' },
  'in Arbeit': { fg: 'var(--color-statusInArbeit)', soft: 'var(--color-statusInArbeitSoft)' },
  fertig: { fg: 'var(--color-statusFertig)', soft: 'var(--color-statusFertigSoft)' },
  abgeholt: { fg: 'var(--color-statusAbgeholt)', soft: 'var(--color-statusAbgeholtSoft)' },
}

function statusBadgeClass(status: OrderStatus): string {
  const slug = status === 'in Arbeit' ? 'in-arbeit' : status
  return `status-badge status-badge--${slug}`
}

function errorMessage(error: unknown): string {
  if (error instanceof ApiError) {
    if (error.status === 429) {
      return 'Zu viele Abfragen. Bitte warten Sie eine Minute und versuchen Sie es erneut.'
    }
    return error.code ? `${error.code}: ${error.message}` : error.message
  }
  return 'Die Abfrage ist fehlgeschlagen. Bitte versuchen Sie es erneut.'
}

function buildPlateQuery(plate: string): string {
  const params = new URLSearchParams()
  params.set('plate', plate.trim().toUpperCase())
  return params.toString()
}

type AlertTone = 'danger' | 'info'

function DangerIcon() {
  return (
    <svg
      className="lookup__alert-icon"
      viewBox="0 0 20 20"
      aria-hidden="true"
      focusable="false"
    >
      <circle cx="10" cy="10" r="8.25" fill="none" stroke="currentColor" strokeWidth="1.5" />
      <path d="M10 5.75v5" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
      <circle cx="10" cy="13.75" r="1" fill="currentColor" />
    </svg>
  )
}

function InfoIcon() {
  return (
    <svg
      className="lookup__alert-icon"
      viewBox="0 0 20 20"
      aria-hidden="true"
      focusable="false"
    >
      <circle cx="10" cy="10" r="8.25" fill="none" stroke="currentColor" strokeWidth="1.5" />
      <path d="M10 9v5.25" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
      <circle cx="10" cy="6.25" r="1" fill="currentColor" />
    </svg>
  )
}

function LookupAlert({ tone, children }: { tone: AlertTone; children: ReactNode }) {
  return (
    <div
      className={`alert alert--${tone} lookup__alert lookup__alert--${tone}`}
      role={tone === 'danger' ? 'alert' : 'status'}
    >
      {tone === 'danger' ? <DangerIcon /> : <InfoIcon />}
      <span>{children}</span>
    </div>
  )
}

interface LookupResult {
  status: PublicOrderStatus
  invoice: Invoice | null
  invoiceError: string | null
}

const LOOKUP_STYLES = `
.lookup__head { display: flex; flex-direction: column; text-align: left; }
.lookup__head .page-subtitle { margin-bottom: 0; }
.lookup__alert { align-items: flex-start; }
.lookup__alert-icon { flex: 0 0 auto; width: 20px; height: 20px; }
.lookup__alert--danger .lookup__alert-icon { color: var(--color-danger); }
.lookup__alert--info .lookup__alert-icon { color: var(--color-accent); }
.lookup__form { width: 100%; max-width: var(--container-narrow); margin-inline: auto; }
.lookup__results { display: flex; flex-direction: column; gap: var(--space-3); width: 100%; max-width: var(--container-content); margin-inline: auto; }
.lookup__meta { display: grid; gap: var(--space-2); margin: 0; }
.lookup__meta div { display: grid; gap: var(--space-0); }
.lookup__meta dt { font-size: 12px; line-height: 16px; color: var(--color-muted); }
.lookup__meta dd { margin: 0; font-size: 14px; line-height: 20px; }
.lookup__plate { font-family: var(--font-mono); font-variant-numeric: tabular-nums; font-size: 14px; font-weight: 500; text-transform: uppercase; background: var(--color-surfaceAlt); border: 1px solid var(--color-border); border-radius: var(--radius-sm); padding: 2px 8px; }
.lookup__steps { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: var(--space-2); }
.lookup__step { display: flex; align-items: center; min-height: 44px; padding: 0 16px; border-radius: var(--radius-pill); font-size: 14px; line-height: 20px; font-weight: 500; border: 1px solid transparent; }
.lookup__step--current { box-shadow: 0 0 0 2px var(--color-accent); }
.lookup__step--future { opacity: 0.6; background: var(--color-surfaceAlt); color: var(--color-muted); }
.lookup__timeline { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: var(--space-3); }
.lookup__timeline-item { position: relative; display: flex; gap: var(--space-3); min-height: 44px; padding-left: 24px; }
.lookup__timeline-item::before { content: ''; position: absolute; left: 5px; top: 18px; bottom: -16px; width: 2px; background: var(--color-border); }
.lookup__timeline-item:last-child::before { display: none; }
.lookup__timeline-dot { position: absolute; left: 0; top: 6px; width: 12px; height: 12px; border-radius: 50%; }
.lookup__timeline-body { display: flex; flex-direction: column; gap: 2px; }
.lookup__timeline-status { font-size: 14px; font-weight: 500; }
.lookup__timeline-time { font-size: 12px; color: var(--color-muted); }
.lookup__invoice-table { width: 100%; border-collapse: collapse; }
.lookup__invoice-table th { text-align: left; font-size: 12px; line-height: 16px; font-weight: 500; color: var(--color-muted); text-transform: uppercase; letter-spacing: 0.02em; padding: 8px 12px; background: var(--color-surfaceAlt); }
.lookup__invoice-table td { padding: 12px; border-bottom: 1px solid var(--color-border); font-size: 14px; }
.lookup__summary { margin-top: var(--space-3); display: flex; flex-direction: column; gap: var(--space-1); }
.lookup__summary-row { display: flex; justify-content: space-between; gap: var(--space-3); font-family: var(--font-mono); font-variant-numeric: tabular-nums; font-size: 14px; }
.lookup__summary-row--total { border-top: 1px solid var(--color-border); padding-top: var(--space-2); font-size: 20px; font-weight: 600; }
@media (min-width: 768px) {
  .lookup__meta { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .lookup__steps { flex-direction: row; flex-wrap: wrap; }
  .lookup__step { flex: 1 1 auto; justify-content: center; }
}
`

export default function OrderLookupPage() {
  const [orderNumber, setOrderNumber] = useState('')
  const [plate, setPlate] = useState('')
  const [touched, setTouched] = useState({ orderNumber: false, plate: false })
  const [submitAttempted, setSubmitAttempted] = useState(false)
  const [loading, setLoading] = useState(false)
  const [notFound, setNotFound] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [result, setResult] = useState<LookupResult | null>(null)

  const orderNumberValid = orderNumber.trim() !== ''
  const plateValid = plate.trim() !== ''
  const orderNumberError = (touched.orderNumber || submitAttempted) && !orderNumberValid
  const plateError = (touched.plate || submitAttempted) && !plateValid

  let lookupAlert: { tone: AlertTone; text: string } | null = null
  if (!loading) {
    if (notFound) {
      lookupAlert = {
        tone: 'danger',
        text: 'Zu dieser Kombination aus Auftragsnummer und Kennzeichen wurde kein Auftrag gefunden. Bitte prüfen Sie Ihre Eingaben und versuchen Sie es erneut.',
      }
    } else if (error) {
      lookupAlert = { tone: 'danger', text: error }
    } else if (result && result.invoice === null && result.invoiceError === null) {
      lookupAlert = { tone: 'info', text: 'Rechnung noch nicht vorhanden.' }
    }
  }

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setSubmitAttempted(true)
    if (!orderNumberValid || !plateValid || loading) {
      return
    }

    setLoading(true)
    setNotFound(false)
    setError(null)
    setResult(null)

    const number = encodeURIComponent(orderNumber.trim())
    const query = buildPlateQuery(plate)

    try {
      const status = await apiFetch<PublicOrderStatus>(`/orders/${number}/status?${query}`)

      let invoice: Invoice | null = null
      let invoiceError: string | null = null
      try {
        invoice = await apiFetch<Invoice>(`/orders/${number}/invoice?${query}`)
      } catch (invoiceCause) {
        if (invoiceCause instanceof ApiError && invoiceCause.status === 404) {
          invoice = null
        } else {
          invoiceError = errorMessage(invoiceCause)
        }
      }

      setResult({ status, invoice, invoiceError })
    } catch (cause) {
      if (cause instanceof ApiError && cause.status === 404) {
        setNotFound(true)
      } else {
        setError(errorMessage(cause))
      }
    } finally {
      setLoading(false)
    }
  }

  return (
    <section className="page-section">
      <style>{LOOKUP_STYLES}</style>

      <div className="lookup__head">
        <h1 className="page-title">Status abrufen</h1>
        <p className="page-subtitle">
          Geben Sie Ihre Auftragsnummer und das Kennzeichen ein, um den aktuellen Stand und die
          Rechnung zu sehen.
        </p>
      </div>

      <Tabs
        ariaLabel="Kundenbereich"
        items={[
          { to: '/', label: 'Status abrufen', end: true },
          { to: '/auftrag', label: 'Termin anfragen' },
        ]}
      />

      <div className="lookup__form">
        <form className="card" onSubmit={handleSubmit} noValidate>
          {lookupAlert ? (
            <LookupAlert tone={lookupAlert.tone}>{lookupAlert.text}</LookupAlert>
          ) : null}

          <div className="field">
            <label className="field__label" htmlFor="lookup-order-number">
              Auftragsnummer
            </label>
            <input
              id="lookup-order-number"
              className="field__input mono"
              type="text"
              value={orderNumber}
              placeholder="z. B. AU-2026-0042"
              aria-invalid={orderNumberError ? true : undefined}
              aria-describedby={orderNumberError ? 'lookup-order-number-error' : undefined}
              onChange={(event) => {
                setOrderNumber(event.target.value)
                setTouched((prev) => ({ ...prev, orderNumber: true }))
              }}
            />
            {orderNumberError ? (
              <p id="lookup-order-number-error" className="field__error" role="alert">
                Bitte geben Sie die Auftragsnummer ein.
              </p>
            ) : null}
          </div>

          <div className="field">
            <label className="field__label" htmlFor="lookup-plate">
              Kennzeichen
            </label>
            <input
              id="lookup-plate"
              className="field__input mono"
              type="text"
              value={plate}
              placeholder="z. B. B-AB 1234"
              aria-invalid={plateError ? true : undefined}
              aria-describedby={plateError ? 'lookup-plate-error' : undefined}
              onChange={(event) => {
                setPlate(event.target.value.toUpperCase())
                setTouched((prev) => ({ ...prev, plate: true }))
              }}
            />
            {plateError ? (
              <p id="lookup-plate-error" className="field__error" role="alert">
                Bitte geben Sie das Kennzeichen ein.
              </p>
            ) : null}
          </div>

          <button type="submit" className="btn btn--primary btn--block" disabled={loading} aria-busy={loading}>
            {loading ? 'Wird gesucht …' : 'Status abrufen'}
          </button>
        </form>
      </div>

      {loading ? (
        <div className="lookup__results">
          <div className="card">
            <p className="muted" role="status">
              Auftrag wird geladen …
            </p>
          </div>
        </div>
      ) : null}

      {!loading && result ? <LookupResultView result={result} /> : null}
    </section>
  )
}

function LookupResultView({ result }: { result: LookupResult }) {
  const { status, invoice, invoiceError } = result
  const currentIndex = ORDER_STATUSES.indexOf(status.status)
  const sortedHistory: HistoryEntry[] = [...status.history].sort(
    (a, b) => new Date(a.changed_at).getTime() - new Date(b.changed_at).getTime(),
  )

  return (
    <div className="lookup__results">
      <div className="card">
        <div className="card__header">
          <div>
            <div className="mono" style={{ fontSize: 20, fontWeight: 600 }}>
              {status.order_number}
            </div>
            <div className="muted" style={{ fontSize: 14 }}>
              Auftragsstatus
            </div>
          </div>
          <span className={statusBadgeClass(status.status)}>{status.status}</span>
        </div>

        <dl className="lookup__meta">
          <div>
            <dt>Fahrzeug</dt>
            <dd>
              {status.vehicle.make} {status.vehicle.model}
              <br />
              <span className="lookup__plate">{status.vehicle.plate}</span>
              <br />
              <span className="muted">{formatMileage(status.vehicle.mileage)}</span>
            </dd>
          </div>
          <div>
            <dt>Problembeschreibung</dt>
            <dd>{status.problem}</dd>
          </div>
        </dl>
      </div>

      <div className="card">
        <div className="card__header">
          <h2 className="card__title">Status</h2>
        </div>
        <ol className="lookup__steps">
          {ORDER_STATUSES.map((step, index) => {
            const tokens = STATUS_TOKENS[step]
            if (index === currentIndex) {
              return (
                <li
                  key={step}
                  className="lookup__step lookup__step--current"
                  aria-current="step"
                  style={{ background: tokens.soft, color: tokens.fg }}
                >
                  {step}
                </li>
              )
            }
            if (index < currentIndex) {
              return (
                <li
                  key={step}
                  className="lookup__step lookup__step--reached"
                  style={{ background: tokens.soft, color: tokens.fg }}
                >
                  {step}
                </li>
              )
            }
            return (
              <li key={step} className="lookup__step lookup__step--future" aria-disabled="true">
                {step}
              </li>
            )
          })}
        </ol>
      </div>

      <div className="card">
        <div className="card__header">
          <h2 className="card__title">Verlauf</h2>
        </div>
        {sortedHistory.length === 0 ? (
          <p className="muted">Kein Verlauf vorhanden.</p>
        ) : (
          <ul className="lookup__timeline">
            {sortedHistory.map((entry, index) => (
              <li
                className="lookup__timeline-item"
                key={`${entry.status}-${entry.changed_at}-${index}`}
              >
                <span
                  className="lookup__timeline-dot"
                  style={{ background: STATUS_TOKENS[entry.status].fg }}
                  aria-hidden="true"
                />
                <span className="lookup__timeline-body">
                  <span className="lookup__timeline-status">{entry.status}</span>
                  <span className="lookup__timeline-time">{formatDateTime(entry.changed_at)}</span>
                </span>
              </li>
            ))}
          </ul>
        )}
      </div>

      {invoiceError ? (
        <div className="alert alert--danger" role="alert">
          <span>{invoiceError}</span>
        </div>
      ) : null}

      {invoice ? <InvoiceCard invoice={invoice} /> : null}
    </div>
  )
}

function InvoiceCard({ invoice }: { invoice: Invoice }) {
  return (
    <div className="card">
      <div className="card__header">
        <div>
          <h2 className="card__title mono">{invoice.invoice_number}</h2>
          <div className="muted" style={{ fontSize: 14 }}>
            Ausgestellt am {formatDate(invoice.issued_at)}
          </div>
        </div>
      </div>

      <table className="lookup__invoice-table">
        <thead>
          <tr>
            <th>Bezeichnung</th>
            <th style={{ textAlign: 'right' }}>Menge</th>
            <th style={{ textAlign: 'right' }}>Einzelpreis</th>
            <th style={{ textAlign: 'right' }}>Summe</th>
          </tr>
        </thead>
        <tbody>
          <tr>
            <td>Arbeitszeit</td>
            <td className="num" style={{ textAlign: 'right' }}>
              {formatHours(invoice.labor_minutes)}
            </td>
            <td />
            <td />
          </tr>
          {invoice.items.map((item, index) => (
            <tr key={`${item.description}-${index}`}>
              <td>{item.description}</td>
              <td className="num" style={{ textAlign: 'right' }}>
                {item.quantity}
              </td>
              <td className="num" style={{ textAlign: 'right' }}>
                {formatMoney(item.unit_price_cents)}
              </td>
              <td className="num" style={{ textAlign: 'right' }}>
                {formatMoney(item.quantity * item.unit_price_cents)}
              </td>
            </tr>
          ))}
        </tbody>
      </table>

      <div className="lookup__summary">
        <div className="lookup__summary-row">
          <span>Netto</span>
          <span>{formatMoney(invoice.net_cents)}</span>
        </div>
        <div className="lookup__summary-row">
          <span>Mehrwertsteuer 19{NON_BREAKING_SPACE}%</span>
          <span>{formatMoney(invoice.vat_cents)}</span>
        </div>
        <div className="lookup__summary-row lookup__summary-row--total">
          <span>Brutto</span>
          <span>{formatMoney(invoice.gross_cents)}</span>
        </div>
      </div>
    </div>
  )
}
