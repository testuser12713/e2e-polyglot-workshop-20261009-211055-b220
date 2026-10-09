import { useCallback, useEffect, useRef, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { ApiError, apiFetch } from '../api/client'
import { NEXT_ORDER_STATUS, ORDER_STATUSES } from '../api/types'
import type {
  HistoryEntry,
  Invoice,
  OrderPosition,
  OrderStatus,
  PositionUpdate,
  StatusUpdateResponse,
  WorkshopOrderDetail,
} from '../api/types'

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
    return error.message
  }
  return 'Ein unerwarteter Fehler ist aufgetreten.'
}

function nextStatusLabel(status: OrderStatus): string {
  return `Auf \u201E${status}\u201C setzen`
}

interface EditablePart {
  id: number
  description: string
  quantity: string
  unit_price_cents: string
}

function isLaborValid(value: string): boolean {
  return /^\d+$/.test(value.trim())
}

function isPartValid(part: EditablePart): boolean {
  return (
    part.description.trim() !== '' &&
    /^\d+$/.test(part.quantity.trim()) &&
    Number(part.quantity) >= 1 &&
    /^\d+$/.test(part.unit_price_cents.trim())
  )
}

const ORDER_DETAIL_STYLES = `
.order-detail__styles-anchor { display: contents; }
.order-detail__meta { display: grid; gap: var(--space-2); margin: 0; }
.order-detail__meta div { display: grid; gap: var(--space-0); }
.order-detail__meta dt { font-size: 12px; line-height: 16px; color: var(--color-muted); }
.order-detail__meta dd { margin: 0; font-size: 14px; line-height: 20px; }
.order-detail__place { font-family: var(--font-mono); font-variant-numeric: tabular-nums; font-size: 14px; font-weight: 500; text-transform: uppercase; background: var(--color-surfaceAlt); border: 1px solid var(--color-border); border-radius: var(--radius-sm); padding: 2px 8px; }
.order-detail__steps { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: var(--space-2); }
.order-detail__step { display: flex; align-items: center; min-height: 44px; padding: 0 16px; border-radius: var(--radius-pill); font-size: 14px; line-height: 20px; font-weight: 500; border: 1px solid transparent; }
.order-detail__step--reached { border-color: transparent; }
.order-detail__step--current { box-shadow: 0 0 0 2px var(--color-accent); }
.order-detail__step--action { cursor: pointer; border: none; font: inherit; color: var(--color-accentFg); background: var(--color-accent); }
.order-detail__step--action:hover { background: var(--color-accentHover); }
.order-detail__step--disabled { opacity: 0.6; cursor: not-allowed; background: var(--color-surfaceAlt); color: var(--color-muted); }
.order-detail__parts { display: flex; flex-direction: column; gap: var(--space-3); }
.order-detail__part { border: 1px solid var(--color-borderStrong); border-radius: var(--radius-md); padding: var(--space-3); display: flex; flex-direction: column; gap: var(--space-2); }
.order-detail__part-grid { display: grid; grid-template-columns: 1fr; gap: var(--space-2); align-items: end; }
.order-detail__part-preview { font-size: 12px; line-height: 16px; color: var(--color-muted); font-family: var(--font-mono); font-variant-numeric: tabular-nums; }
.order-detail__remove { width: 44px; min-width: 44px; height: 44px; padding: 0; }
.order-detail__invoice-table { width: 100%; border-collapse: collapse; }
.order-detail__invoice-table th { text-align: left; font-size: 12px; line-height: 16px; font-weight: 500; color: var(--color-muted); text-transform: uppercase; letter-spacing: 0.02em; padding: 8px 12px; background: var(--color-surfaceAlt); }
.order-detail__invoice-table td { padding: 12px; border-bottom: 1px solid var(--color-border); font-size: 14px; }
.order-detail__summary { margin-top: var(--space-3); display: flex; flex-direction: column; gap: var(--space-1); }
.order-detail__summary-row { display: flex; justify-content: space-between; gap: var(--space-3); font-family: var(--font-mono); font-variant-numeric: tabular-nums; font-size: 14px; }
.order-detail__summary-row--total { border-top: 1px solid var(--color-border); padding-top: var(--space-2); font-size: 20px; font-weight: 600; }
.order-detail__timeline { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: var(--space-3); }
.order-detail__timeline-item { position: relative; display: flex; gap: var(--space-3); min-height: 44px; padding-left: 24px; }
.order-detail__timeline-item::before { content: ''; position: absolute; left: 5px; top: 18px; bottom: -16px; width: 2px; background: var(--color-border); }
.order-detail__timeline-item:last-child::before { display: none; }
.order-detail__timeline-dot { position: absolute; left: 0; top: 6px; width: 12px; height: 12px; border-radius: 50%; }
.order-detail__timeline-body { display: flex; flex-direction: column; gap: 2px; }
.order-detail__timeline-status { font-size: 14px; font-weight: 500; }
.order-detail__timeline-time { font-size: 12px; color: var(--color-muted); }
.order-detail__modal-overlay { position: fixed; inset: 0; background: rgba(17, 24, 38, 0.45); display: flex; align-items: center; justify-content: center; padding: 16px; z-index: 40; }
.order-detail__modal { background: var(--color-surface); border-radius: var(--radius-xl); padding: var(--space-4); max-width: 480px; width: 100%; }
.order-detail__modal-actions { display: flex; flex-direction: column; gap: var(--space-2); margin-top: var(--space-4); }
@media (min-width: 768px) {
  .order-detail__meta { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .order-detail__steps { flex-direction: row; flex-wrap: wrap; }
  .order-detail__step { flex: 1 1 auto; justify-content: center; }
  .order-detail__part-grid { grid-template-columns: 1.4fr 0.7fr 1fr auto; }
  .order-detail__modal-actions { flex-direction: row; justify-content: flex-end; }
}
`

export default function WorkshopOrderDetailPage() {
  const { number = '' } = useParams<{ number: string }>()

  const [detail, setDetail] = useState<WorkshopOrderDetail | null>(null)
  const [loading, setLoading] = useState(true)
  const [notFound, setNotFound] = useState(false)
  const [loadError, setLoadError] = useState<string | null>(null)

  const [laborMinutes, setLaborMinutes] = useState('0')
  const [parts, setParts] = useState<EditablePart[]>([])
  const [touched, setTouched] = useState<Record<string, boolean>>({})
  const [submitAttempted, setSubmitAttempted] = useState(false)
  const [saving, setSaving] = useState(false)
  const [saveError, setSaveError] = useState<string | null>(null)
  const [saveMessage, setSaveMessage] = useState<string | null>(null)

  const [statusBusy, setStatusBusy] = useState(false)
  const [statusError, setStatusError] = useState<string | null>(null)
  const [confirmFertig, setConfirmFertig] = useState(false)

  const nextPartId = useRef(1)

  const initEditor = useCallback((data: WorkshopOrderDetail) => {
    setLaborMinutes(String(data.positions.labor_minutes))
    setParts(
      data.positions.parts.map((part) => ({
        id: nextPartId.current++,
        description: part.description,
        quantity: String(part.quantity),
        unit_price_cents: String(part.unit_price_cents),
      })),
    )
    setTouched({})
    setSubmitAttempted(false)
    setSaveError(null)
    setSaveMessage(null)
  }, [])

  useEffect(() => {
    let active = true
    setLoading(true)
    setNotFound(false)
    setLoadError(null)
    apiFetch<WorkshopOrderDetail>(`/workshop/orders/${number}`)
      .then((data) => {
        if (!active) {
          return
        }
        setDetail(data)
        initEditor(data)
      })
      .catch((error: unknown) => {
        if (!active) {
          return
        }
        if (error instanceof ApiError && error.status === 404) {
          setNotFound(true)
        } else {
          setLoadError(errorMessage(error))
        }
      })
      .finally(() => {
        if (active) {
          setLoading(false)
        }
      })
    return () => {
      active = false
    }
  }, [number, initEditor])

  const markTouched = (key: string) => setTouched((prev) => ({ ...prev, [key]: true }))

  const updatePart = (id: number, patch: Partial<EditablePart>) => {
    setParts((prev) => prev.map((part) => (part.id === id ? { ...part, ...patch } : part)))
  }

  const addPart = () => {
    setParts((prev) => [
      ...prev,
      { id: nextPartId.current++, description: '', quantity: '1', unit_price_cents: '' },
    ])
  }

  const removePart = (id: number) => {
    setParts((prev) => prev.filter((part) => part.id !== id))
  }

  const canAddPart = parts.length === 0 || isPartValid(parts[parts.length - 1])
  const canSave = isLaborValid(laborMinutes) && parts.every(isPartValid)

  const laborError = (touched.labor === true || submitAttempted) && !isLaborValid(laborMinutes)

  const partFieldError = (part: EditablePart, field: 'description' | 'quantity' | 'unit_price_cents') => {
    if (!(touched[`${part.id}:${field}`] === true || submitAttempted)) {
      return null
    }
    if (field === 'description' && part.description.trim() === '') {
      return 'Bezeichnung erforderlich.'
    }
    if (field === 'quantity' && (!/^\d+$/.test(part.quantity.trim()) || Number(part.quantity) < 1)) {
      return 'Menge: ganze Zahl ab 1.'
    }
    if (field === 'unit_price_cents' && !/^\d+$/.test(part.unit_price_cents.trim())) {
      return 'Einzelpreis: ganze Zahl in Cent.'
    }
    return null
  }

  const savePositions = async () => {
    setSubmitAttempted(true)
    if (!canSave || saving) {
      return
    }
    setSaving(true)
    setSaveError(null)
    setSaveMessage(null)
    const payload: PositionUpdate = {
      labor_minutes: Number(laborMinutes),
      parts: parts.map((part) => ({
        description: part.description.trim(),
        quantity: Number(part.quantity),
        unit_price_cents: Number(part.unit_price_cents),
      })),
    }
    try {
      await apiFetch(`/workshop/orders/${number}/positions`, {
        method: 'PUT',
        body: JSON.stringify(payload),
      })
      setDetail((prev) =>
        prev
          ? {
              ...prev,
              positions: { labor_minutes: payload.labor_minutes, parts: payload.parts },
            }
          : prev,
      )
      setSaveMessage('Positionen gespeichert.')
    } catch (error) {
      setSaveError(errorMessage(error))
    } finally {
      setSaving(false)
    }
  }

  const applyStatus = async (target: OrderStatus) => {
    setStatusBusy(true)
    setStatusError(null)
    try {
      const result = await apiFetch<StatusUpdateResponse>(
        `/workshop/orders/${number}/status`,
        { method: 'POST', body: JSON.stringify({ status: target }) },
      )
      setDetail((prev) =>
        prev
          ? { ...prev, order: { ...prev.order, status: result.status }, history: result.history }
          : prev,
      )
    } catch (error) {
      setStatusError(errorMessage(error))
    } finally {
      setStatusBusy(false)
    }
  }

  const handleNextStatus = (target: OrderStatus) => {
    if (target === 'fertig') {
      setConfirmFertig(true)
      return
    }
    void applyStatus(target)
  }

  useEffect(() => {
    if (!confirmFertig) {
      return
    }
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        setConfirmFertig(false)
      }
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [confirmFertig])

  if (loading) {
    return (
      <section className="page-section page-section--legal">
        <style>{ORDER_DETAIL_STYLES}</style>
        <h1 className="page-title">Auftragsdetails</h1>
        <p className="muted" role="status">
          Auftrag wird geladen …
        </p>
      </section>
    )
  }

  if (notFound) {
    return (
      <section className="page-section page-section--legal">
        <style>{ORDER_DETAIL_STYLES}</style>
        <h1 className="page-title">Auftrag nicht gefunden</h1>
        <div className="alert alert--warning" role="alert">
          <span>
            Der Auftrag „{number}“ wurde nicht gefunden. Bitte prüfen Sie die Auftragsnummer in
            der Auftragsliste.
          </span>
        </div>
        <Link to="/werkstatt/auftraege" className="btn btn--secondary">
          Zur Auftragsliste
        </Link>
      </section>
    )
  }

  if (!detail) {
    return (
      <section className="page-section page-section--legal">
        <style>{ORDER_DETAIL_STYLES}</style>
        <h1 className="page-title">Auftragsdetails</h1>
        <div className="alert alert--danger" role="alert">
          <span>{loadError ?? 'Der Auftrag konnte nicht geladen werden.'}</span>
        </div>
        <Link to="/werkstatt/auftraege" className="btn btn--secondary">
          Zur Auftragsliste
        </Link>
      </section>
    )
  }

  const { order, customer, vehicle, history, invoice } = detail
  const currentIndex = ORDER_STATUSES.indexOf(order.status)
  const nextStatus = NEXT_ORDER_STATUS[order.status]
  const sortedHistory: HistoryEntry[] = [...history].sort(
    (a, b) => new Date(a.changed_at).getTime() - new Date(b.changed_at).getTime(),
  )

  return (
    <section className="page-section page-section--legal">
      <style>{ORDER_DETAIL_STYLES}</style>

      <div className="card">
        <div className="card__header">
          <div>
            <div className="mono" style={{ fontSize: 20, fontWeight: 600 }}>
              {order.order_number}
            </div>
            <div className="muted" style={{ fontSize: 14 }}>
              Auftragsdetails
            </div>
          </div>
          <span className={statusBadgeClass(order.status)}>{order.status}</span>
        </div>

        <dl className="order-detail__meta">
          <div>
            <dt>Wunschtermin</dt>
            <dd>{formatDate(order.desired_date)}</dd>
          </div>
          <div>
            <dt>Problembeschreibung</dt>
            <dd>{order.problem}</dd>
          </div>
          <div>
            <dt>Kunde</dt>
            <dd>
              {customer.name}
              <br />
              <span className="muted">{customer.email}</span>
              <br />
              <span className="muted">{customer.phone}</span>
            </dd>
          </div>
          <div>
            <dt>Fahrzeug</dt>
            <dd>
              {vehicle.make} {vehicle.model}
              <br />
              <span className="order-detail__place">{vehicle.plate}</span>
              <br />
              <span className="muted">{formatMileage(vehicle.mileage)}</span>
            </dd>
          </div>
        </dl>
      </div>

      <div className="card">
        <div className="card__header">
          <h2 className="card__title">Status</h2>
        </div>
        {statusError ? (
          <div className="alert alert--danger" role="alert" style={{ marginBottom: 'var(--space-3)' }}>
            <span>{statusError}</span>
          </div>
        ) : null}
        <ol className="order-detail__steps">
          {ORDER_STATUSES.map((status, index) => {
            const tokens = STATUS_TOKENS[status]
            if (index === currentIndex) {
              return (
                <li
                  key={status}
                  className="order-detail__step order-detail__step--reached order-detail__step--current"
                  aria-current="step"
                  style={{ background: tokens.soft, color: tokens.fg }}
                >
                  {status}
                </li>
              )
            }
            if (nextStatus && status === nextStatus && !statusBusy) {
              return (
                <li key={status} style={{ display: 'flex' }}>
                  <button
                    type="button"
                    className="order-detail__step order-detail__step--action"
                    onClick={() => handleNextStatus(status)}
                  >
                    {nextStatusLabel(status)}
                  </button>
                </li>
              )
            }
            if (index < currentIndex) {
              return (
                <li
                  key={status}
                  className="order-detail__step order-detail__step--reached"
                  style={{ background: tokens.soft, color: tokens.fg }}
                >
                  {status}
                </li>
              )
            }
            return (
              <li
                key={status}
                className="order-detail__step order-detail__step--disabled"
                aria-disabled="true"
              >
                {status}
              </li>
            )
          })}
        </ol>
        {statusBusy ? (
          <p className="muted" role="status" style={{ marginTop: 'var(--space-2)' }}>
            Status wird aktualisiert …
          </p>
        ) : null}
      </div>

      <div className="card">
        <div className="card__header">
          <h2 className="card__title">Positionen</h2>
        </div>

        {saveError ? (
          <div className="alert alert--danger" role="alert" style={{ marginBottom: 'var(--space-3)' }}>
            <span>{saveError}</span>
          </div>
        ) : null}
        {saveMessage ? (
          <div className="alert alert--success" role="status" style={{ marginBottom: 'var(--space-3)' }}>
            <span>{saveMessage}</span>
          </div>
        ) : null}

        <div className="field">
          <label className="field__label" htmlFor="order-detail-labor">
            Arbeitszeit
          </label>
          <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-1)' }}>
            <input
              id="order-detail-labor"
              className="field__input"
              type="number"
              min={0}
              step={1}
              inputMode="numeric"
              value={laborMinutes}
              onChange={(event) => {
                setLaborMinutes(event.target.value)
                markTouched('labor')
              }}
            />
            <span className="muted">min</span>
          </div>
          {laborError ? (
            <p className="field__error" role="alert">
              Arbeitszeit: ganze Zahl ab 0.
            </p>
          ) : null}
        </div>

        <h3 style={{ fontSize: 16, marginTop: 'var(--space-4)', marginBottom: 'var(--space-2)' }}>
          Teile
        </h3>

        {parts.length === 0 ? (
          <p className="muted">Noch keine Teile erfasst.</p>
        ) : (
          <div className="order-detail__parts">
            {parts.map((part) => {
              const priceValid = /^\d+$/.test(part.unit_price_cents.trim())
              return (
                <div className="order-detail__part" key={part.id}>
                  <div className="field" style={{ marginBottom: 0 }}>
                    <label className="field__label" htmlFor={`order-detail-part-desc-${part.id}`}>
                      Bezeichnung
                    </label>
                    <input
                      id={`order-detail-part-desc-${part.id}`}
                      className="field__input"
                      type="text"
                      value={part.description}
                      onChange={(event) => {
                        updatePart(part.id, { description: event.target.value })
                        markTouched(`${part.id}:description`)
                      }}
                    />
                    {partFieldError(part, 'description') ? (
                      <p className="field__error" role="alert">
                        {partFieldError(part, 'description')}
                      </p>
                    ) : null}
                  </div>

                  <div className="order-detail__part-grid">
                    <div className="field" style={{ marginBottom: 0 }}>
                      <label className="field__label" htmlFor={`order-detail-part-qty-${part.id}`}>
                        Menge
                      </label>
                      <input
                        id={`order-detail-part-qty-${part.id}`}
                        className="field__input"
                        type="number"
                        min={1}
                        step={1}
                        inputMode="numeric"
                        value={part.quantity}
                        onChange={(event) => {
                          updatePart(part.id, { quantity: event.target.value })
                          markTouched(`${part.id}:quantity`)
                        }}
                      />
                      {partFieldError(part, 'quantity') ? (
                        <p className="field__error" role="alert">
                          {partFieldError(part, 'quantity')}
                        </p>
                      ) : null}
                    </div>

                    <div className="field" style={{ marginBottom: 0 }}>
                      <label className="field__label" htmlFor={`order-detail-part-price-${part.id}`}>
                        Einzelpreis (Cent)
                      </label>
                      <input
                        id={`order-detail-part-price-${part.id}`}
                        className="field__input"
                        type="number"
                        min={0}
                        step={1}
                        inputMode="decimal"
                        value={part.unit_price_cents}
                        onChange={(event) => {
                          updatePart(part.id, { unit_price_cents: event.target.value })
                          markTouched(`${part.id}:unit_price_cents`)
                        }}
                      />
                      <p className="order-detail__part-preview">
                        {priceValid ? formatMoney(Number(part.unit_price_cents)) : '0,00 €'}
                      </p>
                      {partFieldError(part, 'unit_price_cents') ? (
                        <p className="field__error" role="alert">
                          {partFieldError(part, 'unit_price_cents')}
                        </p>
                      ) : null}
                    </div>

                    <button
                      type="button"
                      className="btn btn--secondary order-detail__remove"
                      aria-label="Position entfernen"
                      onClick={() => removePart(part.id)}
                    >
                      ×
                    </button>
                  </div>
                </div>
              )
            })}
          </div>
        )}

        <div style={{ display: 'flex', flexWrap: 'wrap', gap: 'var(--space-2)', marginTop: 'var(--space-3)' }}>
          <button
            type="button"
            className="btn btn--secondary"
            onClick={addPart}
            disabled={!canAddPart}
          >
            Position hinzufügen
          </button>
          <button
            type="button"
            className="btn btn--primary"
            onClick={() => void savePositions()}
            disabled={!canSave || saving}
            aria-busy={saving}
          >
            {saving ? 'Speichern …' : 'Positionen speichern'}
          </button>
        </div>
      </div>

      <div className="card">
        <div className="card__header">
          <h2 className="card__title">Verlauf</h2>
        </div>
        {sortedHistory.length === 0 ? (
          <p className="muted">Kein Verlauf vorhanden.</p>
        ) : (
          <ul className="order-detail__timeline">
            {sortedHistory.map((entry, index) => (
              <li
                className="order-detail__timeline-item"
                key={`${entry.status}-${entry.changed_at}-${index}`}
              >
                <span
                  className="order-detail__timeline-dot"
                  style={{ background: STATUS_TOKENS[entry.status].fg }}
                  aria-hidden="true"
                />
                <span className="order-detail__timeline-body">
                  <span className="order-detail__timeline-status">{entry.status}</span>
                  <span className="order-detail__timeline-time">
                    {formatDateTime(entry.changed_at)}
                  </span>
                </span>
              </li>
            ))}
          </ul>
        )}
      </div>

      {invoice ? (
        <InvoiceCard invoice={invoice} />
      ) : (
        <div className="card">
          <div className="card__header">
            <h2 className="card__title">Rechnung</h2>
          </div>
          <div className="alert alert--info" role="status">
            <span>Keine Rechnung vorhanden. Sie entsteht, sobald der Auftrag fertig ist.</span>
          </div>
        </div>
      )}

      {confirmFertig ? (
        <div className="order-detail__modal-overlay" role="presentation">
          <div
            className="order-detail__modal"
            role="dialog"
            aria-modal="true"
            aria-labelledby="order-detail-confirm-title"
          >
            <h2 id="order-detail-confirm-title" className="card__title">
              Auftrag auf „fertig“ setzen?
            </h2>
            <p style={{ marginTop: 'var(--space-2)' }}>
              Damit wird die Rechnung erzeugt. Dieser Schritt kann nicht zurückgenommen werden.
            </p>
            <div className="order-detail__modal-actions">
              <button
                type="button"
                className="btn btn--secondary"
                onClick={() => setConfirmFertig(false)}
              >
                Abbrechen
              </button>
              <button
                type="button"
                className="btn btn--primary"
                onClick={() => {
                  setConfirmFertig(false)
                  void applyStatus('fertig')
                }}
              >
                Auf „fertig“ setzen
              </button>
            </div>
          </div>
        </div>
      ) : null}
    </section>
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

      <table className="order-detail__invoice-table">
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
          {invoice.items.map((item: OrderPosition, index: number) => (
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

      <div className="order-detail__summary">
        <div className="order-detail__summary-row">
          <span>Netto</span>
          <span>{formatMoney(invoice.net_cents)}</span>
        </div>
        <div className="order-detail__summary-row">
          <span>Mehrwertsteuer 19{NON_BREAKING_SPACE}%</span>
          <span>{formatMoney(invoice.vat_cents)}</span>
        </div>
        <div className="order-detail__summary-row order-detail__summary-row--total">
          <span>Brutto</span>
          <span>{formatMoney(invoice.gross_cents)}</span>
        </div>
      </div>
    </div>
  )
}
