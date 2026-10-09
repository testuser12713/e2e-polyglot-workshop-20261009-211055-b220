export type OrderStatus =
  | 'angefragt'
  | 'bestätigt'
  | 'in Arbeit'
  | 'fertig'
  | 'abgeholt'

export const ORDER_STATUSES: readonly OrderStatus[] = [
  'angefragt',
  'bestätigt',
  'in Arbeit',
  'fertig',
  'abgeholt',
]

export const NEXT_ORDER_STATUS: Partial<Record<OrderStatus, OrderStatus>> = {
  angefragt: 'bestätigt',
  bestätigt: 'in Arbeit',
  'in Arbeit': 'fertig',
  fertig: 'abgeholt',
}

export interface ErrorBody {
  code: string
  message: string
}

export interface Health {
  status: string
  database: string
  queue: string
}

export interface Customer {
  id: number
  name: string
  email: string
  phone: string
}

export interface CustomerInput {
  name: string
  email: string
  phone: string
}

export interface Vehicle {
  plate: string
  make: string
  model: string
  mileage: number
}

export interface VehicleInput {
  plate: string
  make: string
  model: string
  mileage: number
}

export interface AppointmentRequest {
  customer: CustomerInput
  vehicle: VehicleInput
  desired_date: string
  problem: string
}

export interface AppointmentResponse {
  order_number: string
  status: OrderStatus
}

export interface HistoryEntry {
  status: OrderStatus
  changed_at: string
}

export interface PublicOrderStatus {
  order_number: string
  status: OrderStatus
  vehicle: Vehicle
  problem: string
  history: HistoryEntry[]
}

export interface InvoiceItem {
  description: string
  quantity: number
  unit_price_cents: number
}

export interface Invoice {
  invoice_number: string
  issued_at: string
  labor_minutes: number
  items: InvoiceItem[]
  net_cents: number
  vat_cents: number
  gross_cents: number
}

export interface LoginRequest {
  email: string
  password: string
}

export interface Employee {
  email: string
  name: string
}

export interface LoginResponse {
  token: string
  employee: Employee
}

export interface WorkshopOrderSummary {
  order_number: string
  status: OrderStatus
  plate: string
  customer_name: string
}

export interface WorkshopOrderList {
  orders: WorkshopOrderSummary[]
}

export interface OrderPosition {
  description: string
  quantity: number
  unit_price_cents: number
}

export interface WorkshopOrder {
  order_number: string
  status: OrderStatus
  desired_date: string
  problem: string
}

export interface WorkshopOrderDetail {
  order: WorkshopOrder
  customer: Customer
  vehicle: Vehicle
  positions: {
    labor_minutes: number
    parts: OrderPosition[]
  }
  history: HistoryEntry[]
  invoice: Invoice | null
}

export interface PositionUpdate {
  labor_minutes: number
  parts: OrderPosition[]
}

export interface StatusUpdate {
  status: OrderStatus
}

export interface StatusUpdateResponse {
  status: OrderStatus
  history: HistoryEntry[]
}

export interface Dashboard {
  open_orders: number
  finished_today: number
  revenue_month_cents: number
}
