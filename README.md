<!--
  Banner promo: tambahkan di sini nanti, contoh:
  <p align="center"><img src="docs/assets/banner.png" alt="TuPay" width="100%"></p>
-->

<p align="center"><img src="https://cdn.tanet.eu.org/img/tupay_dashboard_v3.png" alt="TuPay" width="100%"></p>
<p align="center"><em>Invoicing &amp; payments, minus the busywork.</em></p>

<p align="center">
  <a href="LICENSE"><img alt="License: AGPL-3.0" src="https://img.shields.io/badge/License-AGPL--3.0-blue.svg"></a>
  <a href="VERSION"><img alt="Version" src="https://img.shields.io/badge/dynamic/regex?url=https%3A%2F%2Fraw.githubusercontent.com%2Ftertua%2Ftupay%2Fdev%2FVERSION&amp;search=(.*)&amp;label=version&amp;color=blue"></a>
  <img alt="Go" src="https://img.shields.io/badge/Go-1.27-00ADD8.svg?logo=go&amp;logoColor=white">
  <img alt="React" src="https://img.shields.io/badge/React-19-61DAFB.svg?logo=react&amp;logoColor=white">
  <img alt="Vite" src="https://img.shields.io/badge/Vite-8-646CFF.svg?logo=vite&amp;logoColor=white">
</p>
<p align="center">
  <a href="https://github.com/tertua/tupay/actions/workflows/web-check.yml"><img alt="Web check" src="https://github.com/tertua/tupay/actions/workflows/web-check.yml/badge.svg"></a>
  <a href="https://github.com/tertua/tupay/actions/workflows/dialect-check.yml"><img alt="Dialect check" src="https://github.com/tertua/tupay/actions/workflows/dialect-check.yml/badge.svg"></a>
  <img alt="PRs welcome" src="https://img.shields.io/badge/PRs-welcome-brightgreen.svg">
</p>

**TuPay** is an invoice management platform for small teams: create invoices, share public payment links, collect online payments, manage clients and expenses, and follow it all from a dashboard with reports — with AI-assisted text in English and Indonesian.

## Features

- **Invoices** — create, send, and track status; download ready-to-share PDFs.
- **Public pay links** — a shareable link and hosted pay page for each invoice.
- **Online payments** — QRIS and crypto methods, switched on once a provider is configured.
- **Clients & expenses** — keep client records and log business expenses.
- **Dashboard & reports** — key metrics and charts, plus a separate admin console.
- **Gateway relay** — API-key-authenticated integration for third-party services.
- **AI-assisted text** — invoice descriptions, payment reminders, and expense notes.

## Tech stack

- **Backend** — Go (Fiber), GORM over SQLite or PostgreSQL, optional Redis.
- **Frontend** — React + Vite multi-page app (product app + admin console), TanStack Query, Tailwind CSS. Built with [Bun](https://bun.sh).
- **API** — Swagger UI at `/swagger/index.html`.

## Quick start

Prerequisites: Docker (easiest), or Go + Bun for local development.

Docker:

```bash
cp .env.example .env
make docker.run
```

Open Swagger: http://127.0.0.1:5000/swagger/index.html

Local dev (no external services required):

```bash
make dev-be   # API on :5000
make dev-fe   # Vite dev server on :5173
```

## Documentation

- [`ARCHITECTURE.md`](ARCHITECTURE.md) — system design, data flow, security, deploy.
- [`CODE_STYLE.md`](CODE_STYLE.md) — conventions, patterns, CI gates.
- [`docs/MODULE_MAP.md`](docs/MODULE_MAP.md) — domain ownership and request path.
- [`docs/API_DOCS.md`](docs/API_DOCS.md) — API conventions.
- [`CONTRIBUTING.md`](CONTRIBUTING.md) — local setup, branching model, and PR process.
- [`SUPPORT.md`](SUPPORT.md) — where to ask questions, report bugs, and request features.
- [`SECURITY.md`](SECURITY.md) — supported versions and private vulnerability reporting.

## License

AGPL-3.0 — see [LICENSE](LICENSE).
