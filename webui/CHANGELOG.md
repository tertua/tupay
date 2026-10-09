# Changelog

All notable changes to this project are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Client receivables tooling: the clients list is now server-driven (search,
  lifecycle status, sort, pagination), clients can be archived/unarchived, an
  archived client can be restored, and deleting a client with open invoices is
  blocked. The client detail gains an overdue-only view, a "Send reminder"
  action that reuses the scheduled reminder outbox path, and a CSV statement
  export.
- Send invoice by email: an owner-only button on the invoice detail emails the
  invoice to its client through the mail outbox, using a dedicated invoice-send
  template. Re-sending is allowed; drafts and clients without an email address
  are rejected with a clear message.

## [v2.5.0] - 2026-10-08

### Added
- Automatic recurring invoices: reusable templates with a weekly or monthly
  cadence generate invoices on schedule through the background worker.
  Generation is claim-guarded, so overlapping runs or restarts never produce
  duplicates, and month-end dates clamp (Jan 31 → Feb 28/29).

### Changed
- The recurring-invoice feature is renamed to Subscriptions end to end. The API
  endpoints moved from `/invoice-templates` to `/subscriptions` (JSON envelope
  keys `invoice_template`/`invoice_templates` → `subscription`/`subscriptions`;
  field names unchanged), the web UI route moved from `/templates` to
  `/subscriptions`, and the database tables were renamed by schema migration 25
  (`invoice_templates` → `subscriptions`, `invoice_template_items` →
  `subscription_items`, `invoice_template_run` → `subscription_runs`).

### Fixed
- The `Makefile` docker targets use `tupay-*` container and network names
  instead of the leftover `template-*` prefix, matching the documented
  `make docker.run` flow.

## [v2.4.2] - 2026-10-07

### Changed
- The invoice editor's payment method is now a single "Online payment"
  checkbox (on = Online, off = Cash) instead of a three-way select. Only
  "Online" changes behaviour, so a boolean no longer reads as a three-way
  choice. New invoices default to Online; existing bank-transfer invoices
  are unaffected, and RecordPaymentModal still offers all three methods.

### Fixed
- The payment reminder sweep no longer logs a duplicate-key error on every
  poll for an already-claimed reminder leg: the claim checks for the existing
  row first, while the unique insert stays as the race guard across replicas.

## [v2.4.1] - 2026-10-04

### Fixed
- Invoice detail carries its line items and payments again. The editor showed an
  empty item list after "Save & send" even though the rows were stored, and a
  follow-up save would have replaced them, because the response omitted `items`
  and `payments`.

### Changed
- The admin Gateway console no longer repeats the gateway availability panel —
  the console header already reports the configured/sandbox/live status.

## [v2.4.0] - 2026-10-03

### Changed
- Project license switched from GPLv3 to AGPLv3: the network interaction
  clause now requires offering source to users who interact with TuPay
  remotely, closing the SaaS loophole.

## [v2.3.1] - 2026-10-02

### Added
- Architecture Decision Records under `docs/adr/`: ADR 0001 defines the
  `INTEGRATION CONTRACT` marker so a deliberate not-yet-used integration
  point is never deleted as dead code, and `AGENTS.md` gains the
  "buka pintu integrasi" command protocol (DB schema + ADR + door stub).

### Fixed
- Module map is back in sync: every controller owner file is referenced
  in its domain row, and the stale `auth_verify.go` entry now points at
  its split files (`auth_verify_handlers.go`, `auth_verify_token.go`).

## [v2.3.0] - 2026-10-02

### Added
- OIDC login can map a configurable role claim (`OIDC_ROLE_CLAIM` /
  `OIDC_ADMIN_ROLE`) to the platform role on sign-in.

### Changed
- The public pay shell no longer shows a language switcher.
- OIDC provider discovery is cached per issuer, and the issuer is
  validated/normalized at startup.

### Fixed
- OIDC provisioning is atomic and transactional: identity linking is
  TOCTOU-safe and role-claim extraction walks the claims payload once.
- WebUI: settlement count renders correctly, icon-only buttons are
  labelled, and the expenses empty-state receipt outline is straightened.
- ID copy keeps familiar English terms.

## [v2.2.0] - 2026-10-01

### Added
- Single sign-on via OIDC: configurable provider, SSO button on the
  login page, state/resolve handshake, and identity linking by verified
  email.
- Email verification on registration and an invite-only registration
  gate — new accounts start pending until verified (or invited).
- Admins can block and unblock user accounts (`PATCH /admin/users/:id/status`):
  the target's live session is revoked on block and the change is
  audit-logged; there is intentionally no hard delete.
- Admins can bootstrap an organization for any owner (`POST /admin/orgs`).
- Mobile bottom navigation, dashboard polish (sparkline, count-up, aging)
  and optimistic list mutations with a shared table skeleton.

### Changed
- Gateway providers are pluggable and provider-neutral end to end (config,
  method defaults, allowlist, checkout dispatch, schema) — adding a
  provider no longer requires core edits.
- Docker images are hardened and built multi-arch.

### Fixed
- Multi-tenant and org-role gaps (#1–#6): org-scoped records can no longer
  leak across tenants or roles.
- Date-only SQL bounds force UTC midnights, keeping report ranges stable
  across timezones.

### Security
- The user/auth/OIDC pipeline is hardened end to end (session, CSRF,
  guards).

## [v2.1.0] - 2026-09-30

### Changed
- The frontend is now built with Bun instead of npm: local dev, CI,
  the all-in-one Docker image and Dependabot all use `bun`, and the
  lockfile is `webui/bun.lock`. Frontend tests still run on Node's
  built-in runner (`node --test`) via `bun run test` — never `bun test`.

## [v2.0.0] - 2026-09-30

### Added
- Organizations: accounts can now be grouped into organizations with
  memberships and email invites, and every org-owned record (invoices,
  clients, expenses, settings) is scoped to its organization — data from
  one org is never visible to another.
- Invoice approval: an org can require approval per role, so invoices
  raised by members are held until an approver releases them; the
  organization settings tab manages members, invites and that policy.
- The gateway now guards project deletion: a project that still has live
  keys or traffic must be confirmed before it can be removed.
- Outbox worker health is exposed for operators (pending/failed counts),
  making stuck delivery jobs visible instead of silent.

### Fixed
- Rate-limit counters no longer leak between limit classes — each class
  (auth, API, public) now keeps its own keys, so a hit on one no longer
  shortens the window of another.
- PostgreSQL automigrate keeps `settings.user_id` NOT NULL, so a fresh
  Postgres install no longer drifts from the SQLite schema.

## [v1.1.0] - 2026-09-28

### Added
- The notification bell is now live: invoice and payment events arrive over
  a server-sent event stream, so the badge updates without a page refresh.
- The public pay page polls the payment status adaptively (fast while a
  payment is pending, slow otherwise) instead of hammering the API.

### Changed
- The product is branded TuPay across the console, emails and API metadata.
- The webhook origin env is renamed to `TUPAY_PUBLIC_URL`; the old
  `INVOICEMAN_PUBLIC_URL` keeps working as a fallback, and the default
  SQLite path moves to `./data/db/tupay.db`.

## [v1.0.0] - 2026-09-28

### Added
- The admin console is now a dedicated single-page entry: `/admin/*` serves
  its own `admin.html` bundle with a separate `admin-*.js` chunk, so the
  console and the product app load, cache and deploy independently. Admin
  links in the sidebar switch entries via a full page load, and a build
  without `admin.html` falls back to the main entry.

### Changed
- The embedded frontend is renamed from SPA to WebUI: deployments set
  `SERVE_WEBUI` (the old `SERVE_SPA_DIR` keeps working as a fallback) and
  the Go plumbing follows suit (`MountWebUI`, `WebUIConfig`).
- The frontend folder moves from `web/` to `webui/`: `make web.check` is now
  `make webui.check`, `npm --prefix web` is `npm --prefix webui`, and CI,
  Docker and docs paths were updated to match.
- Gateway, invoice and outbox internals were consolidated (single route
  type, shared intent-claim and totals helpers, unified not-configured
  sentinel, registry-backed method offering) to prepare for adding more
  payment providers; no behavior change is intended.
- The README was rewritten in English and stale setup content refreshed.

## [v0.7.0] - 2026-09-27

### Added
- The public pay QRIS widget renders the payment code itself from the raw
  EMVCo payload (`qr_string`) now stored on the gateway transaction, so the
  code appears as a self-contained `data:image/png;base64,...` image instead
  of loading the provider's QR image URL. Intents created before this release
  keep the hosted image as a fallback.

### Fixed
- The public pay crypto widget no longer requests a deposit address the
  moment it opens. Picking crypto now shows the asset picker with nothing
  preselected, so the payer must choose an asset; the gateway is called only
  after "Create deposit address", and "Change asset" returns to the picker
  without charging anything.
- The crypto minimum is checked against the asset the payer picked instead of
  always USDT (BSC): `GET /public/pay/:token?pay_currency=` gates the method
  list per asset (no asset chosen = no gate), and the widget preflights that
  minimum before creating a deposit address.

## [v0.6.1] - 2026-09-27

### Fixed
- Correct the USDT BEP20 pay currency: the provider code is `usdtbsc`, not
  `usdtbep20`, so BEP20 payments no longer fail with "currency not found". The
  BEP20 asset is now the default picker choice.

## [v0.6.0] - 2026-09-27

### Added
- Crypto asset picker on the public pay page: payers choose between USDT
  (TRC20/ERC20/BEP20), TRX, DOGE, or LTC, each with its own deposit address.

### Fixed
- Switching crypto assets opens a fresh deposit address instead of reusing
  the first asset's address.
- Unsupported crypto assets are rejected with a clear error.
- Payment finish page handles Midtrans redirects directly.
- Payment submission is idempotent, so retries never double-charge.
- Stale Midtrans orders are failed fast instead of retried forever.

## [v0.5.8] - 2026-09-27

### Added
- Payment method section in the invoice editor and detail, so owners can
  record how an invoice is meant to be paid.

### Changed
- Midtrans is locked to QRIS: the payment method picker is removed from
  owner settings, and the Snap fallback now opens the GoPay QRIS flow
  directly instead of asking the payer to choose again.
- Moderator role management is removed from the UI.
- Online payment UX is simplified, with fewer steps between the invoice
  and the payment page.

### Fixed
- Invoice double-submit is guarded, so a second click cannot create a
  duplicate payment intent.
- Dashboard collection rate no longer breaks when totals arrive as
  non-numeric values.
- The public pay page shows a clearer error when an invoice or payment
  link is not found.

## [v0.5.7] - 2026-09-26

### Fixed
- Client-cancelled or timed-out requests now stop their server-side work
  instead of running to completion: AI text generation, session and CSRF
  lookups, cache invalidation, and retried database queries all follow the
  request context.
- NOWPayments rate-limit retries honor the provider's `Retry-After` header
  and fall back to jittered backoff, so a throttled gateway is not hammered.

### Changed
- Payment-gateway HTTP failures surface as a typed error
  (`gateway.ErrProviderStatus`) whose status code can be inspected with
  `errors.Is`/`errors.As`, instead of matching provider message strings.
- Retry policy and relay response-size limits now live in `pkg/constants`.

## [v0.5.6] - 2026-09-26

### Fixed
- Payment intent waits no longer hang when the payer disconnects
  mid-request: the QRIS/relay claim polls and the idempotency replay wait
  now abort with the request context and back off (100ms doubling, capped
  at 1s) instead of sleeping on a fixed tick.
- Concurrent payment submits are detected reliably on both SQLite and
  PostgreSQL via typed duplicate-key errors instead of message matching.
- Background worker bookkeeping failures (mail/delivery/notification state
  writes, reconcile touches) are now logged with context instead of failing
  silently.

### Changed
- All payment-gateway HTTP calls (Midtrans, NOWPayments, Turnstile, QR
  image fetch) share configured clients with explicit timeouts and
  connection pooling.

## [v0.5.5] - 2026-09-26

### Security
- Webhook settlement now refuses notifications whose amount or currency
  does not match the stored intent, preventing settlement on mismatched
  provider payloads.

### Changed
- The Edit button on paid or pending invoices is now hidden instead of
  shown disabled.
- QRIS and crypto pay widgets use localized strings and dynamic QR
  filenames; gateway HTTP calls identify as `InvoiceMan/<version>` and
  webhook relays as `InvoiceMan-Relay/<version>`.

## [v0.5.4] - 2026-09-26

### Fixed
- The first QRIS tap on a public pay link no longer fails with "payment
  could not be started": concurrent submits now collapse onto a single QR
  instead of racing each other at the gateway.

### Changed
- `POST /gateway/intents` now requires an `Idempotency-Key` header (same as
  `POST /gateway/invoices`), so a retried relay submit replays the original
  intent instead of opening a second payable charge. Send one UUID per
  payment intent.

## [v0.5.3] - 2026-09-25

### Changed
- "Sent" invoice status now uses sky blue instead of teal, so it no longer
  looks almost identical to "Paid" green — in badges, dashboard/Reports
  charts, and notification pills.
- USD → IDR rate hint in company settings shortened to one line; the input
  placeholder already shows the 18000 example.

## [v0.5.2] - 2026-09-25

### Added
- Public pay page embeds a crypto widget: pay with USDT (TRC20) without
  leaving the page — QR code, deposit address with copy button, exact amount
  to send, and a countdown, instead of redirecting to the provider.

### Changed
- Payment start failures now explain themselves: a rate-limited provider
  says it is busy and to retry in a few seconds, and invoices below the
  crypto minimum are told to pick another method. Both translate to English
  and Indonesian.
- Crypto payments are confirmed by the background reconcile poll as well as
  webhooks, so invoices mark themselves paid even where the provider cannot
  reach the server (local sandbox, missed IPN).

### Fixed
- NOWPayments direct payments failed with "price_amount must be a safe
  number" because the amount was sent as a string; the widget could not
  open a transaction at all.
- Rate-limited NOWPayments calls now retry briefly instead of failing the
  first attempt with a generic gateway error.

## [v0.5.1] - 2026-09-25

### Added
- AI invoice notes/terms now forbid thank-you and cooperation closings
  ("thank you", "terima kasih", "kerja sama", "kerjasama", "cooperation").

### Changed
- Gateway methods trimmed to `bank_transfer`, `qris`, `gopay`, `credit_card`;
  the Midtrans setting is a single-select dropdown defaulting to `gopay`.
- NOWPayments invoices are always charged in USD; IDR balances convert with
  the manual `usd_to_idr` rate instead of being sent as-is.
- Auth forms drop stiff example placeholders; login disables browser email
  history and uses `new-password` for the password field; address hints are
  unified to "Alamat Lengkap".
- Indonesian copy shortened: pending status "Menunggu", payment link title
  "Tautan Publik", invoice lines "Qty"/"Harga"; the share-link copy is
  icon-only.
- Settings merges the Account and Password tabs; the sidebar logout moves
  into the user card (avatar is display-only, only the logout icon acts).
- Dashboard dates honor the saved language from first paint, so ID renders
  "24 Sep 2026" instead of "Sep 24, 2026".
- Public pay secured-by note reads "Pembayaran diproses aman oleh mitra kami".

## [v0.5.0] - 2026-09-24

### Added
- Choose which Midtrans payment methods your account offers. The setting lists
  every method Midtrans supports; unchecking one hides it from payment links
  and blocks it if requested directly. Leaving all checked keeps the previous
  "every method" behaviour.

### Changed
- The public pay page now shows each method as a name-only button. The invoice
  total is already shown above, so the per-button converted amount that
  overlapped on every method is gone.

## [v0.4.0] - 2026-09-24

### Added
- Multi-provider payments: a `payment_method` on an intent now routes to any
  configured provider that supports it, regardless of the project default. The
  optional `gateway` field still pins a provider for legacy callers.
- Manual USD/IDR conversion: a new `usd_to_idr` setting (IDR per 1 USD) lets an
  IDR-only provider charge a USD invoice and vice versa. No realtime FX feed is
  used; when the rate is unset the intent is rejected instead of guessing.
- The public pay page now lets the payer choose a payment method first and only
  then opens the gateway paylink (Snap for Midtrans, hosted checkout for
  crypto), showing the converted amount per method.

### Changed
- Gateway transactions record the charged currency plus the source
  `invoice_currency`/`invoice_amount` and the `usd_to_idr` rate used.
- Settings gain the manual rate field; the public "secured by" note is now
  provider-neutral.

## [v0.3.1] - 2026-09-24

### Fixed
- The local Snap intent for an invoice works again. It moved out of the
  API-key `/gateway` namespace to `POST /api/v1/invoices/{id}/intents`, so a
  signed-in session is no longer rejected with "missing api key".
- Provider webhooks no longer register a duplicate route: `/webhooks/midtrans`
  and `/webhooks/nowpayments` are both served by the single generic
  `/webhooks/{gateway}` handler.

### Changed
- `payment_method` is validated against the known method ids and
  `GET /gateway/methods` now returns a typed `{ id, name }` list. The optional
  `gateway` field is documented as a legacy override; new integrations should
  send only `payment_method`.

## [v0.3.0] - 2026-09-24

### Added
- Provider-neutral payment methods: payment intents accept an optional
  `payment_method` (`qris`, `bank_transfer`, `gopay`, `crypto`, ...), routed to
  a provider that supports it, and a new `GET /api/v1/gateway/methods` lists
  the available methods without exposing provider names.
- Gateway transactions persist the chosen `payment_method`, expose it on intent
  responses, and forward it in the relay webhook payload.

### Changed
- Relay webhook payload keeps the provider-specific `payment_type` and now also
  carries the neutral `payment_method`; `gross_amount_idr` is an integer number
  of fiat minor units.

### Fixed
- Money no longer round-trips through `float64` on the gateway path: NOWPayments
  prices and verifies amounts as exact decimals and local settlement builds the
  amount from integer minor units, so fractional amounts stay precise.

## [v0.2.3] - 2026-09-24

### Added
- External service projects can create invoices through the API key-protected
  `/api/v1/gateway/invoices` endpoint with an inline customer payload.
- Integration invoice creation supports project-scoped external IDs and
  idempotent retries through the `Idempotency-Key` header.
- Swagger documentation and integration flow tests cover customer creation,
  invoice creation, retries, and duplicate external IDs.

### Added
- Sending an invoice now requires a client: the backend returns `422 client is
  required to send an invoice`, the detail page hides the "Sent" action, and the
  editor blocks the save. Drafts may still be saved without a client.
- Frontend unit tests (`npm test`, Node's built-in runner) and a `check:charts`
  guard that fails when a `<Pie>` renders un-coerced money.
- Flow tests build invoices through the shared `invoiceSpec` fixture
  (`flow_fixtures_test.go`); `check:fixtures` fails on hand-written invoice JSON.

### Fixed
- Status donuts on the dashboard and reports render again: money arrives as
  decimal strings, which Recharts' `Pie` ignores when summing slices.

## [v0.2.2] - 2026-09-24

### Added
- Awaiting-payment (`pending`) is a first-class display status: a live gateway
  transaction overlays the stored status on the dashboard, reports, client
  list/detail and the public pay page, and gets its own slice in the status
  donut.
- Public pay page shows a distinct "Awaiting payment" badge while a Snap intent
  is live, and keeps its Pay button so the in-flight intent can be resumed.

### Changed
- Money in flight locks the invoice: content edits, status flips, hard delete,
  manual payment recording, voiding and creating a new payment link all return
  `422 invoice has a pending payment`. An existing public link stays usable and
  visible.
- Pending invoices count as receivables (outstanding / totalBilled) in
  dashboard, reports and client aggregates; a stale draft with a live intent
  reads as payable instead of "still a draft".
- `payment_controller.go` split into `payment_public_controller.go`; public
  draft gates go through a shared `effectiveInvoiceStatus` helper.

### Fixed
- Dashboard/report/client outstanding no longer drops invoices whose stored
  status is draft while a gateway payment is pending.
- The status donut no longer hides the pending slice.
- Public pay and online-link endpoints no longer reject a pending invoice as
  "still a draft".

## [v0.2.1] - 2026-09-24

### Added
- Awaiting-payment status with gateway reconciliation: live transactions
  surface as pending and stale ones are settled by the outbox worker.
- Invoices get empty default terms; AI notes read as money wisdom.

### Changed
- Money uses the `models.Money` decimal alias consistently, avoiding frontend
  precision loss.

### Fixed
- Parse Midtrans `gross_amount` as decimal instead of float64.

## [v0.2.0] - 2026-09-24

### Added
- Per-user language persists in the database; session caches warm during the
  login delay.
- `dev-be` / `dev-fe` make targets for the local dev run.

### Changed
- Money is stored and returned as fixed-point decimals.
- Unified design tokens, radius, and focus states.
- Project license switched to GPLv3.

## [v0.1.7] - 2026-09-23

### Changed
- Oversized files split; file-size budgets enforced in CI.

### Fixed
- Auto-mark invoices paid once payments cover the total, syncing client
  aggregates.
- Lock paid invoices and persist the payment link display.
- Strip sensitive fields from public payment data.
- Use the `PAY-` prefix for local order ids.
- Force AI terms output to a numbered list only.

## [v0.1.6] - 2026-09-23

### Added
- Public gateway status endpoint; gateway availability banner and gateway
  select in the SPA.

### Changed
- Payments are voided instead of hard-deleted; gateway-settled payments hide
  the void action.
- Hide the AI reminder once an invoice is paid.

### Fixed
- NOWPayments decimal guard.

## [v0.1.5] - 2026-09-23

### Added
- User webhook targets for invoice and payment events.
- AI text follows the UI language and user currency; Gemini rate limits map to
  HTTP 429; business summary and payment-reminder drafts persist in
  localStorage.
- Browser language detection for first-time visitors.

### Changed
- React 19.3 and Vite 8.3.

### Fixed
- Settle local invoice payments atomically.
- Correct the inverted custom UUID validator rule; fix the aging chart measure.
- Preserve line breaks in invoice notes and terms.
- Count only sent invoices as outstanding.

## [v0.1.4] - 2026-09-22

### Added
- Registration kill switch via `ALLOW_REGISTRATION`.
- Back-to-home link on the login page.
- Optional embedded SPA via `SERVE_SPA_DIR` and `Dockerfile.dev`.

## [v0.1.3] - 2026-09-22

### Added
- Auto-redirect to login on an expired session; query error states with retry.

### Security
- Stop baking `.env` into the Docker image.
- Serve only public logos under `/uploads`; receipts stay behind the auth proxy.
- Store password-reset tokens hashed.
- Reject SVG uploads; serve legacy SVG downloads as attachment.
- Strict single-session, atomic first-admin, CSRF rotation, proxy trust and
  login audit; secure cookie clear and HS256 pin.

## [v0.1.2] - 2026-09-22

### Added
- Per-domain module map with `check:map` and CI enforcement.
- Self-contained Swagger (session cookie scheme, auth flows, envelopes).
- Dual-backend parity: cache, retry, rollback and guards.

### Changed
- Docker build context excludes `web`, `data`, and local artifacts.

## [v0.1.1] - 2026-09-22

### Added
- Database-backed cookie sessions, RBAC, and an admin users UI.
- Clients, invoices, dashboard, catalog items, expenses, payments and reports.
- Public payment links and a public pay page with language toggle and `?lang=`.
- Central Midtrans relay, generalized into a multi-gateway framework (Midtrans
  Snap, NOWPayments).
- Generic SMTP email delivery.
- Platform hardening: recover/requestid/helmet/rate-limit/health probes and
  SIGTERM shutdown, typed config + slog + Prometheus metrics, pagination +
  idempotency keys + outbox worker, API versioning + CSRF/audit + Turnstile +
  version guard, S3-compatible storage and HTML mail templates.
- Route-level error pages, bundle splitting and budgets, EN/ID localization of
  backend errors, currency-aware dashboard totals.

### Changed
- Brand renamed to Invoiceman with `APP_NAME` override.
- Feature modules split; the API client was renamed to `http`.

### Fixed
- Fail fast on an unsupported `SQL_DSN` instead of silently falling back to
  SQLite.
- Accessibility: pointer cursor on buttons, keyboard-accessible cards and rows,
  Escape closes modals, buttons default to `type="button"`.
- Localize AR aging buckets and monthly chart labels via stable backend keys.

## [v0.1.0] - 2026-09-20

Initial commit. Baseline scaffold: a Vite + React 19 + Tailwind v4 SPA over a
Go (Fiber + GORM) API with SQLite/PostgreSQL support, cookie-session auth,
invoice CRUD, clients, dashboard, reports, and PDF export.
