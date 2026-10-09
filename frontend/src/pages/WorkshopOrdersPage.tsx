import { useEffect, useState } from 'react'
import type { FormEvent, KeyboardEvent } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { ApiError, apiFetch } from '../api/client'
import { ORDER_STATUSES } from '../api/types'
import type { OrderStatus, WorkshopOrderSummary } from '../api/types'

const STATUS_BADGE_CLASS: Record<OrderStatus, string> = {
  angefragt: 'status-badge--angefragt',
  bestätigt: 'status-badge--bestätigt',
  'in Arbeit': 'status-badge--in-arbeit',
  fertig: 'status-badge--fertig',
  abgeholt: 'status-badge--abgeholt',
}

/** Builds the `/workshop/orders` path with only the filters that are set. */
export function buildOrdersQuery(status: string, plate: string): string {
  const params = new URLSearchParams()
  if (status) {
    params.set('status', status)
  }
  const trimmedPlate = plate.trim()
  if (trimmedPlate) {
    params.set('plate', trimmedPlate.toUpperCase())
  }
  const query = params.toString()
  return query ? `/workshop/orders?${query}` : '/workshop/orders'
}

function errorMessage(error: unknown): string {
  if (error instanceof ApiError) {
    if (error.status === 401) {
      return 'Die Anmeldung ist abgelaufen. Bitte melden Sie sich erneut an.'
    }
    return error.code ? `${error.code}: ${error.message}` : error.message
  }
  return 'Die Aufträge konnten nicht geladen werden. Bitte versuchen Sie es erneut.'
}

export default function WorkshopOrdersPage() {
  const navigate = useNavigate()
  const [status, setStatus] = useState('')
  const [plateInput, setPlateInput] = useState('')
  const [appliedPlate, setAppliedPlate] = useState('')
  const [orders, setOrders] = useState<WorkshopOrderSummary[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [reloadKey, setReloadKey] = useState(0)

  useEffect(() => {
    let active = true
    setLoading(true)
    setError(null)
    apiFetch<{ orders: WorkshopOrderSummary[] }>(buildOrdersQuery(status, appliedPlate))
      .then((data) => {
        if (active) {
          setOrders(data.orders ?? [])
        }
      })
      .catch((cause: unknown) => {
        if (active) {
          setOrders([])
          setError(errorMessage(cause))
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
  }, [status, appliedPlate, reloadKey])

  const openOrder = (orderNumber: string) => {
    navigate(`/werkstatt/auftraege/${encodeURIComponent(orderNumber)}`)
  }

  const handleRowKeyDown = (event: KeyboardEvent<HTMLTableRowElement>, orderNumber: string) => {
    if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault()
      openOrder(orderNumber)
    }
  }

  const handleSearch = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setAppliedPlate(plateInput)
    setReloadKey((key) => key + 1)
  }

  const handleStatusChange = (value: string) => {
    setStatus(value)
    setReloadKey((key) => key + 1)
  }

  return (
    <section className="page-section">
      <style>{`
        .orders-filters {
          display: flex;
          flex-wrap: wrap;
          align-items: flex-end;
          gap: var(--space-3);
        }
        .orders-filters__field {
          display: flex;
          flex-direction: column;
          gap: var(--space-1);
          flex: 1 1 220px;
        }
        .orders-filters__label {
          font-size: 14px;
          line-height: 20px;
          font-weight: 500;
          color: var(--color-fg);
        }
        .orders-table tbody tr {
          cursor: pointer;
        }
        .orders-table tbody tr:hover {
          background: #fafbfd;
        }
        .orders-table__number {
          color: var(--color-accent);
        }
        @media (max-width: 767px) {
          .orders-table,
          .orders-table tbody,
          .orders-table tr,
          .orders-table td {
            display: block;
            width: 100%;
          }
          .orders-table thead {
            display: none;
          }
          .orders-table tr {
            border: 1px solid var(--color-border);
            border-radius: var(--radius-lg);
            background: var(--color-surface);
            padding: var(--space-3);
            margin-bottom: var(--space-3);
          }
          .orders-table td {
            border-bottom: none;
            padding: 2px 0;
            display: flex;
            justify-content: space-between;
            gap: var(--space-2);
          }
          .orders-table td::before {
            content: attr(data-label);
            font-size: 12px;
            line-height: 16px;
            color: var(--color-muted);
          }
        }
      `}</style>

      <h1 className="page-title">Aufträge</h1>
      <p className="page-subtitle">
        Alle Werkstattaufträge im Überblick. Nach Status filtern oder das Kennzeichen suchen.
      </p>

      <form className="card orders-filters" onSubmit={handleSearch} role="search">
        <div className="orders-filters__field">
          <label className="orders-filters__label" htmlFor="orders-status">
            Status
          </label>
          <select
            id="orders-status"
            className="field__select"
            value={status}
            onChange={(event) => handleStatusChange(event.target.value)}
          >
            <option value="">Alle Status</option>
            {ORDER_STATUSES.map((option) => (
              <option key={option} value={option}>
                {option}
              </option>
            ))}
          </select>
        </div>

        <div className="orders-filters__field">
          <label className="orders-filters__label" htmlFor="orders-plate">
            Kennzeichen
          </label>
          <input
            id="orders-plate"
            className="field__input mono"
            type="search"
            value={plateInput}
            placeholder="z. B. B-AB 1234"
            onChange={(event) => setPlateInput(event.target.value.toUpperCase())}
          />
        </div>

        <button type="submit" className="btn btn--primary">
          Suchen
        </button>
      </form>

      {error ? (
        <div className="alert alert--danger" role="alert">
          <span>{error}</span>
        </div>
      ) : null}

      {loading ? (
        <div className="card">
          <p className="muted" role="status">
            Aufträge werden geladen …
          </p>
        </div>
      ) : null}

      {!loading && !error && orders.length === 0 ? (
        <div className="card">
          <div className="empty-state">
            <h2 className="empty-state__title">Keine Aufträge gefunden</h2>
            <p className="empty-state__text">
              Für die gewählten Filter liegen keine Aufträge vor. Passen Sie Status oder Kennzeichen
              an und suchen Sie erneut.
            </p>
          </div>
        </div>
      ) : null}

      {!loading && !error && orders.length > 0 ? (
        <div className="card">
          <table className="table orders-table">
            <thead>
              <tr>
                <th scope="col">Auftragsnummer</th>
                <th scope="col">Status</th>
                <th scope="col">Kennzeichen</th>
                <th scope="col">Kunde</th>
              </tr>
            </thead>
            <tbody>
              {orders.map((order) => (
                <tr
                  key={order.order_number}
                  tabIndex={0}
                  onClick={() => openOrder(order.order_number)}
                  onKeyDown={(event) => handleRowKeyDown(event, order.order_number)}
                >
                  <td data-label="Auftragsnummer">
                    <Link
                      to={`/werkstatt/auftraege/${encodeURIComponent(order.order_number)}`}
                      className="mono orders-table__number"
                      onClick={(event) => event.stopPropagation()}
                    >
                      {order.order_number}
                    </Link>
                  </td>
                  <td data-label="Status">
                    <span className={`status-badge ${STATUS_BADGE_CLASS[order.status]}`}>
                      {order.status}
                    </span>
                  </td>
                  <td data-label="Kennzeichen">
                    <span className="mono">{order.plate}</span>
                  </td>
                  <td data-label="Kunde">{order.customer_name}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : null}
    </section>
  )
}
