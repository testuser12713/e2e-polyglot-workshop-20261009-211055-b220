import { useState } from 'react'
import type { FormEvent } from 'react'
import { de } from 'date-fns/locale'
import { format, startOfToday } from 'date-fns'
import { DayPicker } from 'react-day-picker'
import 'react-day-picker/style.css'
import { ApiError, apiFetch } from '../api/client'
import type { AppointmentRequest, AppointmentResponse } from '../api/types'
import Tabs from '../components/Tabs'

type FieldName = 'name' | 'email' | 'phone' | 'plate' | 'make' | 'model' | 'mileage' | 'problem'
type FieldKey = FieldName | 'desired_date'

interface FormValues {
  name: string
  email: string
  phone: string
  plate: string
  make: string
  model: string
  mileage: string
  problem: string
}

const EMPTY_FORM: FormValues = {
  name: '',
  email: '',
  phone: '',
  plate: '',
  make: '',
  model: '',
  mileage: '',
  problem: '',
}

const EMAIL_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]+$/
const GERMAN_DATE_PATTERN = /^(\d{1,2})\.(\d{1,2})\.(\d{4})$/

const AREA_TABS = [
  { to: '/', label: 'Status abrufen', end: true },
  { to: '/auftrag', label: 'Termin anfragen' },
]

// parseGermanDate turns a typed "TT.MM.JJJJ" value into a Date. It returns
// undefined for anything that is not a real calendar date, so an incomplete or
// impossible entry simply leaves the field without a selection and the regular
// required-date message applies.
function parseGermanDate(value: string): Date | undefined {
  const match = GERMAN_DATE_PATTERN.exec(value.trim())
  if (!match) return undefined
  const day = Number(match[1])
  const month = Number(match[2])
  const year = Number(match[3])
  const parsed = new Date(year, month - 1, day)
  if (
    parsed.getFullYear() !== year ||
    parsed.getMonth() !== month - 1 ||
    parsed.getDate() !== day
  ) {
    return undefined
  }
  return parsed
}

function validate(values: FormValues, date: Date | undefined): Partial<Record<FieldKey, string>> {
  const errors: Partial<Record<FieldKey, string>> = {}
  if (!values.name.trim()) errors.name = 'Bitte geben Sie Ihren Namen ein.'
  if (!values.email.trim()) errors.email = 'Bitte geben Sie Ihre E-Mail-Adresse ein.'
  else if (!EMAIL_PATTERN.test(values.email.trim())) {
    errors.email = 'Bitte geben Sie eine gültige E-Mail-Adresse ein.'
  }
  if (!values.phone.trim()) errors.phone = 'Bitte geben Sie Ihre Telefonnummer ein.'
  if (!values.plate.trim()) errors.plate = 'Bitte geben Sie das Kennzeichen ein.'
  if (!values.make.trim()) errors.make = 'Bitte geben Sie die Marke ein.'
  if (!values.model.trim()) errors.model = 'Bitte geben Sie das Modell ein.'
  if (!values.mileage.trim()) errors.mileage = 'Bitte geben Sie den Kilometerstand ein.'
  else if (!/^\d+$/.test(values.mileage.trim())) {
    errors.mileage = 'Bitte geben Sie den Kilometerstand als ganze Zahl in km ein.'
  }
  if (!date) errors.desired_date = 'Bitte wählen Sie einen Wunschtermin aus.'
  if (!values.problem.trim()) errors.problem = 'Bitte beschreiben Sie das Problem.'
  return errors
}

interface TextFieldProps {
  id: string
  label: string
  value: string
  error?: string
  type?: string
  inputMode?: 'text' | 'email' | 'tel' | 'numeric' | 'decimal'
  autoComplete?: string
  mono?: boolean
  onChange: (value: string) => void
  onBlur: () => void
}

function TextField({
  id,
  label,
  value,
  error,
  type = 'text',
  inputMode,
  autoComplete,
  mono,
  onChange,
  onBlur,
}: TextFieldProps) {
  const errorId = `${id}-error`
  return (
    <div className="field">
      <label className="field__label" htmlFor={id}>
        {label}
      </label>
      <input
        id={id}
        className={mono ? 'field__input mono' : 'field__input'}
        type={type}
        inputMode={inputMode}
        autoComplete={autoComplete}
        value={value}
        aria-invalid={error ? true : undefined}
        aria-describedby={error ? errorId : undefined}
        onChange={(event) => onChange(event.target.value)}
        onBlur={onBlur}
      />
      {error && (
        <span className="field__error" id={errorId} role="alert">
          {error}
        </span>
      )}
    </div>
  )
}

const datePickerStyles = `
.appointment-page {
  gap: 0;
}
.appointment-head {
  display: flex;
  flex-direction: column;
  gap: var(--space-1);
}
.appointment-head .page-subtitle {
  margin-bottom: 0;
}
.appointment-container {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
  width: 100%;
  max-width: var(--container-narrow);
  margin-top: var(--space-5);
}
.appointment-form {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
}
.appointment-datepicker .rdp-root {
  --rdp-accent-color: var(--color-accent);
  --rdp-accent-background-color: var(--color-accentSoft);
  --rdp-today-color: var(--color-accent);
  --rdp-day-height: 44px;
  --rdp-day-width: 44px;
  --rdp-day_button-border-radius: var(--radius-md);
  --rdp-day_button-border: 1px solid transparent;
  --rdp-outside-opacity: 0.6;
  --rdp-disabled-opacity: 0.5;
  font-size: 14px;
  padding: var(--space-2) 0;
}
.appointment-datepicker .rdp-day_button {
  border-radius: var(--radius-md);
  font-size: 14px;
}
.appointment-datepicker .rdp-day_button:hover {
  background: var(--color-accentSoft);
}
.appointment-datepicker .rdp-selected .rdp-day_button {
  background: var(--color-accent);
  color: var(--color-accentFg);
}
.appointment-datepicker .rdp-today:not(.rdp-selected) .rdp-day_button {
  border-color: var(--color-accent);
}
.appointment-datepicker .rdp-weekday {
  font-size: 12px;
  color: var(--color-muted);
  font-weight: 500;
}
.appointment-datepicker .rdp-caption_label {
  font-weight: 600;
}
.appointment-datepicker__row {
  display: flex;
  align-items: stretch;
  gap: var(--space-1);
}
.appointment-datepicker__row .field__input {
  flex: 1 1 auto;
}
.appointment-datepicker__toggle {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex: 0 0 auto;
  width: 44px;
  min-height: 44px;
  padding: 0;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  background: var(--color-surface);
  color: var(--color-muted);
  cursor: pointer;
}
.appointment-datepicker__toggle:hover {
  background: var(--color-surfaceAlt);
  border-color: var(--color-borderStrong);
}
.appointment-datepicker__calendar {
  margin-top: var(--space-1);
  padding: var(--space-2);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  background: var(--color-surface);
}
.appointment-order {
  font-family: var(--font-mono);
  font-variant-numeric: tabular-nums;
  font-size: 30px;
  line-height: 38px;
  font-weight: 600;
  color: var(--color-accent);
  word-break: break-all;
}
.appointment-actions {
  margin-top: var(--space-3);
}
@media (min-width: 768px) {
  .appointment-datepicker .rdp-root {
    --rdp-day-height: 40px;
    --rdp-day-width: 40px;
  }
}
`

export default function AppointmentPage() {
  const [values, setValues] = useState<FormValues>(EMPTY_FORM)
  const [date, setDate] = useState<Date | undefined>(undefined)
  const [dateText, setDateText] = useState('')
  const [calendarOpen, setCalendarOpen] = useState(false)
  const [touched, setTouched] = useState<Partial<Record<FieldKey, boolean>>>({})
  const [submitted, setSubmitted] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [apiError, setApiError] = useState<{ code: string; message: string } | null>(null)
  const [orderNumber, setOrderNumber] = useState<string | null>(null)

  const errors = validate(values, date)
  const showError = (key: FieldKey): string | undefined =>
    submitted || touched[key] ? errors[key] : undefined

  const setField = (key: FieldName) => (value: string) => {
    setValues((current) => ({ ...current, [key]: value }))
  }
  const touch = (key: FieldKey) => () => {
    setTouched((current) => ({ ...current, [key]: true }))
  }

  const openCalendar = () => {
    setTouched((current) => ({ ...current, desired_date: true }))
    setCalendarOpen(true)
  }

  const handleDateText = (value: string) => {
    setDateText(value)
    setDate(parseGermanDate(value))
  }

  const selectDate = (day: Date | undefined) => {
    setDate(day)
    setDateText(day ? format(day, 'dd.MM.yyyy') : '')
    setCalendarOpen(false)
  }

  const resetForm = () => {
    setValues(EMPTY_FORM)
    setDate(undefined)
    setDateText('')
    setCalendarOpen(false)
    setTouched({})
    setSubmitted(false)
    setApiError(null)
    setOrderNumber(null)
  }

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setSubmitted(true)
    setApiError(null)
    const nextErrors = validate(values, date)
    if (Object.keys(nextErrors).length > 0) {
      return
    }

    const payload: AppointmentRequest = {
      customer: {
        name: values.name.trim(),
        email: values.email.trim(),
        phone: values.phone.trim(),
      },
      vehicle: {
        plate: values.plate.trim().toUpperCase(),
        make: values.make.trim(),
        model: values.model.trim(),
        mileage: Number(values.mileage.trim()),
      },
      desired_date: format(date as Date, 'yyyy-MM-dd'),
      problem: values.problem.trim(),
    }

    setSubmitting(true)
    try {
      const response = await apiFetch<AppointmentResponse>('/appointments', {
        method: 'POST',
        body: JSON.stringify(payload),
      })
      setOrderNumber(response.order_number)
      setValues(EMPTY_FORM)
      setDate(undefined)
      setDateText('')
      setTouched({})
      setSubmitted(false)
      setCalendarOpen(false)
    } catch (error) {
      if (error instanceof ApiError) {
        setApiError({ code: error.code, message: error.message })
      } else {
        setApiError({
          code: 'unexpected_error',
          message: 'Die Terminanfrage konnte nicht gesendet werden. Bitte versuchen Sie es erneut.',
        })
      }
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <section className="page-section appointment-page">
      <style>{datePickerStyles}</style>

      <div className="appointment-head">
        <h1 className="page-title">Terminanfrage</h1>
        <Tabs items={AREA_TABS} />
        <p className="page-subtitle">
          Teilen Sie uns Ihre Kontakt- und Fahrzeugdaten mit. Wir melden uns zur Bestätigung des
          Wunschtermins.
        </p>
      </div>

      <div className="appointment-container">
        {orderNumber && (
          <div className="card" role="status" aria-live="polite">
            <h2 className="card__title">Anfrage eingegangen</h2>
            <p>
              Vielen Dank! Ihre Terminanfrage wurde übermittelt. Bitte notieren Sie sich Ihre
              Auftragsnummer:
            </p>
            <p className="appointment-order" aria-label="Auftragsnummer">
              {orderNumber}
            </p>
            <p className="muted">
              Mit dieser Nummer und dem Kennzeichen können Sie den Status Ihres Auftrags jederzeit
              abrufen.
            </p>
            <div className="appointment-actions">
              <button type="button" className="btn btn--secondary" onClick={resetForm}>
                Weitere Terminanfrage
              </button>
            </div>
          </div>
        )}

        {!orderNumber && (
          <form className="appointment-form" onSubmit={handleSubmit} noValidate>
            {apiError && (
              <div className="alert alert--danger" role="alert">
                <div>
                  <strong>{apiError.message}</strong>
                  <div className="muted">Fehlercode: {apiError.code}</div>
                </div>
              </div>
            )}

            <div className="card">
              <h2 className="card__title">Ihre Kontaktdaten</h2>
              <TextField
                id="appointment-name"
                label="Name"
                value={values.name}
                error={showError('name')}
                autoComplete="name"
                onChange={setField('name')}
                onBlur={touch('name')}
              />
              <TextField
                id="appointment-email"
                label="E-Mail"
                type="email"
                inputMode="email"
                autoComplete="email"
                value={values.email}
                error={showError('email')}
                onChange={setField('email')}
                onBlur={touch('email')}
              />
              <TextField
                id="appointment-phone"
                label="Telefon"
                type="tel"
                inputMode="tel"
                autoComplete="tel"
                value={values.phone}
                error={showError('phone')}
                onChange={setField('phone')}
                onBlur={touch('phone')}
              />
            </div>

            <div className="card">
              <h2 className="card__title">Ihr Fahrzeug</h2>
              <TextField
                id="appointment-plate"
                label="Kennzeichen"
                value={values.plate}
                error={showError('plate')}
                autoComplete="off"
                mono
                onChange={setField('plate')}
                onBlur={touch('plate')}
              />
              <TextField
                id="appointment-make"
                label="Marke"
                value={values.make}
                error={showError('make')}
                autoComplete="off"
                onChange={setField('make')}
                onBlur={touch('make')}
              />
              <TextField
                id="appointment-model"
                label="Modell"
                value={values.model}
                error={showError('model')}
                autoComplete="off"
                onChange={setField('model')}
                onBlur={touch('model')}
              />
              <TextField
                id="appointment-mileage"
                label="Kilometerstand (km)"
                inputMode="numeric"
                value={values.mileage}
                error={showError('mileage')}
                autoComplete="off"
                mono
                onChange={setField('mileage')}
                onBlur={touch('mileage')}
              />
            </div>

            <div className="card">
              <h2 className="card__title">Wunschtermin und Problem</h2>

              <div className="field appointment-datepicker">
                <label className="field__label" htmlFor="appointment-date">
                  Wunschtermin
                </label>
                <div className="appointment-datepicker__row">
                  <input
                    id="appointment-date"
                    className="field__input"
                    type="text"
                    inputMode="numeric"
                    autoComplete="off"
                    placeholder="TT.MM.JJJJ"
                    value={dateText}
                    aria-haspopup="dialog"
                    aria-expanded={calendarOpen}
                    aria-invalid={showError('desired_date') ? true : undefined}
                    aria-describedby={
                      showError('desired_date') ? 'appointment-date-error' : undefined
                    }
                    onChange={(event) => handleDateText(event.target.value)}
                    onClick={openCalendar}
                    onBlur={touch('desired_date')}
                  />
                  <button
                    type="button"
                    className="appointment-datepicker__toggle"
                    aria-label="Kalender öffnen"
                    aria-haspopup="dialog"
                    aria-expanded={calendarOpen}
                    onClick={() => {
                      setTouched((current) => ({ ...current, desired_date: true }))
                      setCalendarOpen((open) => !open)
                    }}
                  >
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
                      <rect x="3" y="4" width="18" height="18" rx="2" />
                      <path d="M16 2v4M8 2v4M3 10h18" />
                    </svg>
                  </button>
                </div>
                {showError('desired_date') && (
                  <span className="field__error" id="appointment-date-error" role="alert">
                    {showError('desired_date')}
                  </span>
                )}

                {calendarOpen && (
                  <div className="appointment-datepicker__calendar">
                    <DayPicker
                      mode="single"
                      locale={de}
                      weekStartsOn={1}
                      selected={date}
                      defaultMonth={date ?? startOfToday()}
                      disabled={{ before: startOfToday() }}
                      onSelect={selectDate}
                    />
                  </div>
                )}
              </div>

              <div className="field">
                <label className="field__label" htmlFor="appointment-problem">
                  Problembeschreibung
                </label>
                <textarea
                  id="appointment-problem"
                  className="field__input"
                  rows={4}
                  value={values.problem}
                  aria-invalid={showError('problem') ? true : undefined}
                  aria-describedby={
                    showError('problem') ? 'appointment-problem-error' : undefined
                  }
                  onChange={(event) => setField('problem')(event.target.value)}
                  onBlur={touch('problem')}
                />
                {showError('problem') && (
                  <span className="field__error" id="appointment-problem-error" role="alert">
                    {showError('problem')}
                  </span>
                )}
              </div>
            </div>

            <button
              type="submit"
              className="btn btn--primary btn--block"
              disabled={submitting}
              aria-busy={submitting}
            >
              {submitting ? 'Wird gesendet …' : 'Termin anfragen'}
            </button>
          </form>
        )}
      </div>
    </section>
  )
}
