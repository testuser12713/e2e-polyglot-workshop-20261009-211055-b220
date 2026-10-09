# Kfz-Werkstatt-Kundenportal

Ein Kundenportal für eine Kfz-Werkstatt: Kundinnen und Kunden fragen online einen
Termin an, verfolgen den Status ihres Werkstattauftrags und sehen die fertige
Rechnung ein. Die Werkstatt verwaltet Aufträge, erfasst Positionen, setzt den
Status entlang eines festen Workflows und sieht ein Dashboard mit den offenen
Aufträgen und dem Monatsumsatz.

Das Produkt besteht aus drei Diensten: einer Go-API auf PostgreSQL, einem
Python-Worker, der Rechnungen aus einer Valkey-Warteschlange erzeugt, und einer
React-Web-App.

## Tech-Stack

- **api** (dieser Dienst): Go, `net/http` aus der Standardbibliothek, `pgx/pgxpool`
- **worker**: Python, `psycopg`, `redis-py` (folgt in einem eigenen Ticket)
- **web**: Vite + React + TypeScript (folgt in einem eigenen Ticket)
- **datenbank**: PostgreSQL 18
- **warteschlange**: Valkey 9.1
- **konfiguration**: Umgebungsvariablen, `RUN.json`, `compose.yaml`

## Voraussetzungen

- Go (aktuelle Version)
- Docker (für PostgreSQL und Valkey beim lokalen Entwickeln)

## Datenbanken starten

Für einen frischen Checkout startet `compose.yaml` beide Datenspeicher mit den
offiziellen Images:

```bash
docker compose up -d
```

Das startet PostgreSQL auf Port `5432` und Valkey auf Port `6379`.

## Konfiguration / Umgebungsvariablen

Die API wird ausschließlich über Umgebungsvariablen konfiguriert. Werte, die zum
Start nötig sind, stehen außerdem in `RUN.json`.

| Variable           | Pflicht | Bedeutung                                                        |
| ------------------ | ------- | --------------------------------------------------------------- |
| `DATABASE_URL`     | ja      | PostgreSQL-DSN, z. B. `postgres://workshop@localhost:5432/workshop?sslmode=disable` |
| `VALKEY_URL`       | ja      | Valkey-URL, z. B. `redis://localhost:6379/0`                    |
| `AUTH_SECRET`      | ja      | Signaturschlüssel für die JWT-Tokens (kein Literal im Repository) |
| `PORT`             | nein    | HTTP-Port, Standard `8000`                                      |
| `HOUR_RATE_CENTS`  | nein    | Stundensatz in ganzen Cent, Standard `6500`                     |
| `FRONTEND_ORIGIN`  | nein    | Erlaubte CORS-Origin, Standard `http://localhost:5173`          |

Ein Start-Skript, das die nötigen Variablen setzt (das Geheimnis wird bei jedem
Start neu erzeugt und landet nie im Repository):

```bash
docker compose up -d

export DATABASE_URL="postgres://workshop@localhost:5432/workshop?sslmode=disable"
export VALKEY_URL="redis://localhost:6379/0"
export AUTH_SECRET="$(openssl rand -hex 32)"
export FRONTEND_ORIGIN="http://localhost:5173"
export PORT=8000
export HOUR_RATE_CENTS=6500

cd backend
go mod download
go run ./cmd/api
```

Beim Start wendet die API automatisch `backend/migrations/0001_init.sql` an und
legt den ersten Mitarbeiter an, sofern noch keiner existiert. Ein manuelles
Anlegen des Schemas ist nicht nötig.

## Entwicklung und Build

```bash
cd backend
go build ./...        # kompilieren
go test ./...         # Tests (brauchen PostgreSQL und Valkey)
go vet ./...          # statische Prüfung
gofmt -l .            # Formatierung prüfen
```

## API-Endpunkte

Basis ist immer `/api`. Fehlerantworten haben einheitlich die Form
`{"code": "<string>", "message": "<string>"}`. Alle Routen unter `/api/workshop/*`
verlangen einen Bearer-Token; ohne gültiges Token antworten sie mit `401`.

### Kundenbereich (öffentlich)

| Methode | Pfad                              | Körper                                                        | Antwort |
| ------- | --------------------------------- | ------------------------------------------------------------- | ------- |
| GET     | `/api/health`                     | –                                                             | `200 {"status","database","queue"}` |
| POST    | `/api/customers`                  | `{"name","email","phone"}`                                    | `201 {id,name,email,phone}` \| `400` |
| GET     | `/api/customers/{id}`             | –                                                             | `200` \| `404` |
| POST    | `/api/vehicles`                   | `{"plate","make","model","mileage"}`                          | `201` \| `400` \| `409` |
| POST    | `/api/appointments`               | `{"customer","vehicle","desired_date","problem"}`             | `201 {order_number,status}` \| `400` |
| GET     | `/api/orders/{number}/status`     | Query `?plate=`                                               | `200 {order_number,status,vehicle,problem,history}` \| `404` \| `429` |
| GET     | `/api/orders/{number}/invoice`    | Query `?plate=`                                               | `200 {invoice_number,issued_at,labor_minutes,items,net_cents,vat_cents,gross_cents}` \| `404` \| `429` |
| POST    | `/api/auth/login`                 | `{"email","password"}`                                        | `200 {token,employee:{email,name}}` \| `401` \| `429` |

### Werkstattbereich (Bearer-Token erforderlich)

| Methode | Pfad                                              | Körper                                                                 | Antwort |
| ------- | ------------------------------------------------- | ---------------------------------------------------------------------- | ------- |
| GET     | `/api/workshop/orders`                            | Query `?status=&plate=`                                                 | `200 {orders:[...]}` \| `401` |
| GET     | `/api/workshop/orders/{number}`                   | –                                                                       | `200 {order,customer,vehicle,positions,history,invoice}` \| `401` \| `404` |
| PUT     | `/api/workshop/orders/{number}/positions`         | `{"labor_minutes","parts":[{"description","quantity","unit_price_cents"}]}` | `200` \| `400` \| `401` |
| POST    | `/api/workshop/orders/{number}/status`            | `{"status"}`                                                            | `200 {status,history}` \| `401` \| `404` \| `409` |
| GET     | `/api/workshop/dashboard`                         | –                                                                       | `200 {open_orders,finished_today,revenue_month_cents}` \| `401` |

Alle Geldbeträge werden als ganze Cent übertragen. Der Statusworkflow ist streng
gerichtet: `angefragt → bestätigt → in Arbeit → fertig → abgeholt`, nur jeweils
ein Schritt vorwärts. Beim Wechsel auf `fertig` legt die API genau eine Nachricht
`{"order_id": <int>}` in die Valkey-Liste `invoices`.

## Funktionsumfang

- Einheitliches JSON-Fehlerformat für 400, 401, 404, 405, 409, 429, 500 und 501
- CORS ausschließlich für die konfigurierte Frontend-Origin
- Bearer-Token (JWT HS256) für alle `/api/workshop/*`-Routen
- Rate-Limits: 10 Login-Versuche und 20 öffentliche Abfragen pro Minute und Client
- PostgreSQL-Schema mit automatischer Migration beim Start
- Valkey-Warteschlange für den Rechnungslauf

> Hinweis: Dieser Stand ist das API-Skelett. Einzelne Fachbereiche antworten noch
> mit `501 Not Implemented` (im einheitlichen Fehlerformat), bis ihr jeweiliges
> Ticket umgesetzt ist.
