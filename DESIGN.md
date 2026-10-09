# Design — Project Identity

> This document is project-long-lived. Tokens are not changed without
> the Architect's approval. Developers MUST use these tokens
> instead of improvising their own colors/spacings.

## Style Direction

Calm, professional light UI: near-white surfaces, one deep trustworthy blue accent, system fonts only, hierarchy carried by spacing and a consistent status colour band instead of decorative effects — a workshop that reads as reliable, not playful.

## Colors

- `--color-bg`: **#F5F7FA**
- `--color-surface`: **#FFFFFF**
- `--color-surfaceAlt`: **#EEF1F6**
- `--color-fg`: **#111826**
- `--color-muted`: **#5A6472**
- `--color-border`: **#DFE3EA**
- `--color-borderStrong`: **#C4CBD6**
- `--color-accent`: **#17509B**
- `--color-accentHover`: **#123F79**
- `--color-accentActive`: **#0E3160**
- `--color-accentSoft`: **#E8EFF9**
- `--color-accentFg`: **#FFFFFF**
- `--color-focus`: **#2F7BD6**
- `--color-success`: **#157F3D**
- `--color-successSoft`: **#E4F3E9**
- `--color-warning`: **#A85A08**
- `--color-warningSoft`: **#FBEFDD**
- `--color-danger`: **#B3261E**
- `--color-dangerSoft`: **#FBE7E5**
- `--color-neutralSoft`: **#E7EAEF**
- `--color-statusAng`: **#5A6472**
- `--color-statusAngSoft`: **#E7EAEF**
- `--color-statusBestaetigt`: **#17509B**
- `--color-statusBestaetigtSoft`: **#E8EFF9**
- `--color-statusInArbeit`: **#A85A08**
- `--color-statusInArbeitSoft`: **#FBEFDD**
- `--color-statusFertig`: **#157F3D**
- `--color-statusFertigSoft`: **#E4F3E9**
- `--color-statusAbgeholt`: **#3F4855**
- `--color-statusAbgeholtSoft`: **#E1E5EB**

## Typography

- `font_family`: system-ui, -apple-system, "Segoe UI", Roboto, "Helvetica Neue", Arial, "Noto Sans", sans-serif
- `font_mono`: ui-monospace, "SFMono-Regular", Menlo, Consolas, "Liberation Mono", monospace
- `heading_weight`: 600
- `body_weight`: 400
- `label_weight`: 500
- `size_xs`: 12px/16px
- `size_sm`: 14px/20px
- `size_base`: 16px/24px
- `size_lg`: 20px/28px
- `size_xl`: 24px/32px
- `size_2xl`: 30px/38px
- `size_3xl`: 38px/46px
- `rule`: No webfonts, no font CDN (AC-34) — system stack only; all numeric/monetary cells and identifiers use font-variant-numeric: tabular-nums.

## Spacing Scale

- `--space-0`: 4px
- `--space-1`: 8px
- `--space-2`: 12px
- `--space-3`: 16px
- `--space-4`: 24px
- `--space-5`: 32px
- `--space-6`: 48px

## Border-Radii

- `--radius-sm`: 4px
- `--radius-md`: 8px
- `--radius-lg`: 12px
- `--radius-xl`: 16px
- `--radius-pill`: 999px

## Components

### Button

Variants: primary (bg=accent #17509B, fg=#FFFFFF), secondary (bg=surface #FFFFFF, fg=fg #111826, border 1px borderStrong #C4CBD6), ghost (transparent, fg=accent, border none), danger (bg=danger #B3261E, fg #FFFFFF). Sizes: md = padding 12px 20px, min-height 44px, min-width 44px; sm = padding 8px 14px, min-height 36px (desktop only, never the only control on mobile); full-width variant for mobile forms. Radius md (8px), gap between icon and label 8px, label size_base 16px, weight label_weight 500, no letter-spacing, no shadow. States: default as above; hover = bg darkened one step (primary accentHover #123F79, secondary bg surfaceAlt #EEF1F6, ghost bg accentSoft #E8EFF9, danger #93201A) + transform none; active = one further step down (primary accentActive #0E3160, secondary bg #E1E5EB) + translateY(1px); focus-visible = 2px outline focus #2F7BD6 with 2px offset, always visible on all variants; disabled = opacity 0.6, cursor not-allowed, pointer-events none, no hover change; loading = label replaced by spinner, aria-busy=true, control stays non-interactive. Touch target always >= 44px in the mobile breakpoints (360–767px).

### IconButton

Square 44x44px (36x36px from 768px up), radius md, bg transparent, fg=muted #5A6472, hover bg surfaceAlt, active bg neutralSoft #E7EAEF, focus-visible 2px outline focus, disabled opacity 0.6. Always carries aria-label; never used without one.

### TextField

Label above field: size_sm 14px, weight label_weight 500, color fg, margin-bottom 8px. Input: bg surface #FFFFFF, border 1px border #DFE3EA, radius md 8px, padding 12px 14px, min-height 44px, font size_base 16px (prevents iOS zoom), fg #111826, placeholder color muted #5A6472. Optional hint line below in size_xs 12px, color muted. States: default; hover border borderStrong; focus border 2px accent #17509B + outline 2px focus #2F7BD6 at 2px offset; error = border danger #B3261E, error text 12px below in danger with role=alert, aria-invalid=true, aria-describedby points at the error id; disabled = bg surfaceAlt, fg muted, cursor not-allowed; readonly = bg surfaceAlt, border border. Validation timing (binding): error messages appear only after the field has been touched (blur after typing) or after a submit attempt — never on the untouched form. Beträge/number fields use font_mono with tabular-nums and inputmode=decimal.

### Select

Same box metrics as TextField (44px min-height, radius md, border #DFE3EA). Native <select> on mobile for reliable touch behaviour; custom shell from 768px up with a chevron icon 16px in muted. Statusfilter uses exactly the five workflow labels (siehe Statuswahl). States and error handling identical to TextField; the label of the chosen option is always visible in the closed state.

### Statuswahl (workflow stepper control)

The only control that changes a job status. Rendered as a horizontal (>=768px) / vertical (360–767px) sequence of exactly the five allowed steps angefragt → bestätigt → in Arbeit → fertig → abgeholt. Reached steps: filled pill, colour per step (angefragt #5A6472/soft #E7EAEF, bestätigt #17509B/#E8EFF9, in Arbeit #A85A08/#FBEFDD, fertig #157F3D/#E4F3E9, abgeholt #3F4855/#E1E5EB), height 44px, radius pill, label size_sm weight 500, focus-visible 2px outline focus. The current step is marked with a 2px accent ring and aria-current=step. The single next step is the only enabled button, labelled 'Auf „<Zielstatus>“ setzen', min-height 44px. Every other transition is rendered as a visibly disabled step (opacity 0.6, aria-disabled=true, no hover) and is never offered as an action — no illegal transition exists in the UI (AC-25).

### DatePicker (Wunschtermin)

Wrapper around react-day-picker + date-fns, German locale, week starts Monday, weekday headers Mo–So in size_xs muted. Day cell 44x44px on mobile / 40x40px from 768px up, radius md, size_sm. States: default fg #111826; hover bg accentSoft #E8EFF9; selected bg accent #17509B fg #FFFFFF; today = 1px accent border, no fill; outside-month days muted at 60% opacity; disabled (past) = muted, cursor not-allowed, aria-disabled. The field above the calendar shows the chosen date as '04.06.2026' (TT.MM.JJJJ) and is reachable/typable as a text input as well. All date handling in the app is UTC internally, displayed in Europe/Berlin.

### Card

bg surface #FFFFFF, border 1px border #DFE3EA, radius lg 12px, padding 24px (16px at 360–767px), no shadow by default. Optional header row: title size_lg 20px weight 600, optional right-aligned action; 16px gap below the header. Card grid gap = 16px mobile / 24px desktop. Use Card, not shadows, to separate content — exactly one elevation level exists in this product.

### StatusBadge

Inline pill, height 24px, padding 2px 10px, radius pill, size_xs 12px weight 500, uppercase? no — labels stay as written ('in Arbeit'). Colour pairs from the status palette: angefragt #5A6472 on #E7EAEF, bestätigt #17509B on #E8EFF9, in Arbeit #A85A08 on #FBEFDD, fertig #157F3D on #E4F3E9, abgeholt #3F4855 on #E1E5EB. Contrast of text on its soft background is >= 4.5:1; the badge never relies on colour alone — the status word is always written out.

### Timeline (Statusprotokoll)

Vertical list, one row per logged transition: 12px dot in the step's status colour on the left, 2px vertical connector in border #DFE3EA between rows, content = status label (size_sm weight 500) above the timestamp in size_xs muted. Row min-height 44px, gap 16px between rows, sorted oldest first. Timestamps always printed as '04.06.2026, 14:30 Uhr'.

### Table (Auftragsliste, Positionen)

Desktop >=768px: real table, sticky header row with bg surfaceAlt, header size_xs 12px weight 500 muted uppercase letterspacing 0.02em, rows 48px min-height, 1px bottom border border, hover bg #FAFBFD, whole row clickable (target >= 44px) with focus-visible outline. Numeric columns (Menge, Einzelpreis, Arbeitszeit, Summen) right-aligned, font_mono, tabular-nums. Mobile 360–767px: each row collapses into a Card with label/value pairs (label muted 12px, value 14px), so no horizontal scrolling is ever required.

### LineItemsEditor (Positionserfassung)

Section inside the job detail. Two blocks: 'Arbeitszeit' (single field, Minuten, integer, suffix 'min', min 0, step 1, inputmode=numeric) and 'Teile' (repeatable row: Bezeichnung text, Menge integer >=1, Einzelpreis in ganzen Cent with inputmode=decimal and a live '1.234,56 €' preview, plus a 44x44px remove IconButton). 'Position hinzufügen' — 'Part' primary at the bottom of the list, disabled while the current row is invalid. Unsaved rows show a borderStrong outline; invalid values render the field error inline after touch/submit only, and the save Button stays disabled until all values are valid. Every amount travels and is stored as an integer number of cents.

### TopNav

Full-width bar, height 64px desktop / 56px mobile, bg surface #FFFFFF, 1px bottom border #DFE3EA, content inside the max-width container. Left: wordmark (size_lg 20px weight 600, fg) plus area name. Right >=768px: inline links 'Kundenbereich' / 'Werkstattbereich' and, when logged in, the employee name and 'Abmelden'. Mobile 360–767px: links collapse into a 44x44px menu button (aria-expanded, aria-controls) that opens a full-width panel with 48px rows. Active area link: fg + 2px accent underline. Every page also carries footer links to 'Impressum' and 'Datenschutzerklärung' (AC-33).

### Tabs (Kundenbereich / Werkstattbereich)

Used inside an area to switch subviews (e.g. 'Termin anfragen' / 'Status abrufen' / 'Rechnung'). Row of buttons, height 44px, padding 0 16px, size_sm weight 500, color muted; active = color fg + 2px accent bottom border; hover = bg surfaceAlt; focus-visible 2px outline focus. Directly below the row a 1px border #DFE3EA runs the full width. On 360–767px the row scrolls horizontally with scroll-snap and 16px end padding instead of wrapping.

### LoginForm (Werkstattbereich)

Centered Card, width 100% max 400px, padding 32px (24px mobile), radius lg. Title 'Anmeldung Werkstattbereich' size_xl 24px weight 600, then E-Mail + Passwort TextFields (full width, 44px, password type with reveal IconButton), then the primary submit Button full width. On failure an Alert (danger) appears above the form with a single generic message — never revealing whether the user or the password was wrong. While the request runs the submit Button is loading and both fields are disabled. A 429 answer surfaces as an Alert with the hint to wait a minute; the form keeps its values, the password field is cleared.

### StatTile (Dashboard)

Card with 24px padding: label above in size_sm muted ('Offene Aufträge', 'Heute fertig geworden', 'Umsatz laufender Monat'), then the value in size_2xl 30px weight 600 tabular-nums. Money value printed as '12.345,67 €'. Optional sub-line size_xs muted for context. Grid: 1 column at 360px, 2 columns from 640px, 3 columns from 1024px, gap 16px. No charts, no sparklines — three numbers with clear labels.

### Alert / InlineNotice

Full-width inline block, radius md, padding 12px 16px, border 1px of the tone's base colour on its soft background, icon 20px on the left, text size_sm. Tones: info (accentSoft/border accent, fg #111826), success (#E4F3E9, border #157F3D), warning (#FBEFDD, border #A85A08), danger (#FBE7E5, border #B3261E). role=alert for danger and warning, role=status for info and success. Used for API error bodies (Code + Nachricht), for 'Keine Rechnung vorhanden', for the status-lookup mismatch, and for the generic login failure.

### EmptyState

Centered block inside a Card or page section, padding 48px 24px, max-width 480px: 24px heading in fg, one line of size_sm muted explanation below, and exactly one primary action if the user can do something about it (e.g. 'Weitere Aufträge anzeigen', 'Status erneut abrufen'), else no button at all. Never a decorative illustration — a single muted 32px outline icon at most.

### Modal / ConfirmDialog

Overlay rgba(17,24,38,0.45), panel bg surface, radius xl 16px, padding 24px, max-width 480px, centered, width calc(100% - 32px) at 360px. Focus is trapped inside, Escape closes, focus returns to the trigger. Title size_lg weight 600, body size_base, footer right-aligned with secondary 'Abbrechen' and primary confirm Button; on mobile the footer stacks with the primary Button on top and full width. Used sparingly — only for the irreversible-ish action of setting status 'fertig' (it triggers the invoice run).

### InvoiceView (Rechnung)

Read-only Card: header with 'Rechnung RE-2026-0001' (mono, tabular-nums), issue date as '04.06.2026' and a StatusBadge 'bezahlt' is out of scope — no payment status shown. Below: position table (Bezeichnung | Menge | Einzelpreis | Summe), Arbeitszeit row rendered as '1,50 h × 65,00 €'. Then a right-aligned summary block, each line label left / amount right in mono tabular-nums: 'Netto 123,45 €', 'Mehrwertsteuer 19 % 23,46 €', 'Brutto 146,91 €' separated by a 1px border above the Brutto line in size_lg weight 600. All values come from integer cents and are printed with exactly two decimals; the customer view and the workshop view render amounts identically.

### PlateBadge (Kennzeichen)

Kennzeichen is always displayed uppercase and hyphenated as stored ('B-AB 1234'), in font_mono with tabular-nums, size_sm weight 500, bg surfaceAlt, border 1px border, radius sm, padding 2px 8px. It is the primary identifier in the workshop list, so it sits left of the customer name in every list row and is copyable as plain text. Search input for Kennzeichen uppercases input as you type.

## Layout Principles

- Container max-width 1200px, centered, horizontal padding 16px at 360–767px / 24px at 768–1279px / 32px from 1280px up. Narrow reading/form container 480px (login, Terminanfrage, Statusabfrage), content container 720px for detail pages, 1200px for lists and dashboard.
- Breakpoints: 360px base (small phone, must be fully usable, never horizontal scroll), 480px, 768px (tablet / table → card switch), 1024px (dashboard 3 columns), 1280px (max container padding). Build mobile-first; a layout is only accepted if every screen works at 360px width with 44px touch targets.
- Section rhythm: 48px between page sections on desktop, 32px on mobile; 24px between cards in a grid (16px mobile); 16px between a label and its field, 12px within a field group; page title (size_2xl/3xl, weight 600) followed by 8px and a size_base muted subtitle, then 32px before the first card.
- One elevation level only: separate content with 1px #DFE3EA borders and #FFFFFF / #F5F7FA surfaces. No drop shadows except on Modal and the mobile menu panel. No gradients, no glassmorphism, no animation beyond 120ms ease-out colour/background transitions.
- Area navigation: a persistent TopNav switches between Kundenbereich and Werkstattbereich; the workshop area additionally shows the employee name and 'Abmelden' and renders every page behind the auth check — there is no view where workshop data is painted before the token is verified.
- FORMAT RULES — identical in customer and workshop area, and never re-implemented per ticket. MONEY: stored and transmitted as integer cents; displayed as '1.234,56 €' (de-DE: dot thousands, comma decimals, always exactly two decimals, non-breaking space before €), negative as '−1.234,56 €'; mono, tabular-nums, right-aligned in tables. PERCENT: '19 %' with non-breaking space. DATE: '04.06.2026' (TT.MM.JJJJ). DATE+TIME: '04.06.2026, 14:30 Uhr' (Europe/Berlin display, UTC storage). TIME: '14:30 Uhr'. DURATION: '90 min'; in invoice positions the billed time as '1,50 h' (two decimals, comma). DISTANCE: '123.456 km'. IDENTIFIERS: Auftragsnummer 'AU-2026-0042', Rechnungsnummer 'RE-2026-0001', always mono with tabular-nums and never abbreviated or reformatted. PLATE: uppercase, 'B-AB 1234'. PHONE: as entered, e.g. '030 1234567'. Every placeholder amount, date or duration visible in the UI uses these rules, so parallel tickets cannot diverge.
- Feedback placement: API errors render as an Alert at the top of the affected form or card with the API's code and message; field-level errors render directly under the field. Validation messages never appear in an untouched form (only after touch or submit). Success confirmations are inline near the action (e.g. 'Auftragsnummer AU-2026-0042' after a Terminanfrage), not toast-only, because the number must stay readable and copyable.
- Disabled vs. hidden: an action the user may not perform right now is rendered visibly disabled with an explained reason (aria-disabled, tooltip/hint line) rather than removed — explicitly the illegal status transitions (AC-25) and form submits with invalid input. Purely irrelevant content (e.g. logout without a session) is hidden.
- Accessibility baseline for every component: visible 2px #2F7BD6 focus ring on all interactive elements, contrast >= 4.5:1 for text on its background (status colours are only used as soft backgrounds with dark text, never as text on the page background alone), status is always written out as a word next to its colour, touch targets >= 44x44px, form labels are real <label> elements, and every icon-only control carries an aria-label.
- Privacy-clean by construction: no webfont, script, image or analytics request to any third-party host — only system font stacks, inline SVG icons and same-origin assets (AC-34). Body text size_base 16px minimum on mobile; the only text below 14px is metadata at 12px in muted colour.
