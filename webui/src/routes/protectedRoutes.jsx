import { lazy } from "react";

const Dashboard = lazy(() => import("@/pages/Dashboard"));
const Invoices = lazy(() => import("@/pages/Invoices"));
const InvoiceTemplates = lazy(() => import("@/pages/InvoiceTemplates"));
const InvoiceEditor = lazy(() => import("@/pages/InvoiceEditor"));
const InvoiceDetail = lazy(() => import("@/pages/InvoiceDetail"));
const Clients = lazy(() => import("@/pages/Clients"));
const ClientDetail = lazy(() => import("@/pages/ClientDetail"));
const Expenses = lazy(() => import("@/pages/Expenses"));
const Payments = lazy(() => import("@/pages/Payments"));
const Items = lazy(() => import("@/pages/Items"));
const Reports = lazy(() => import("@/pages/Reports"));
const Settings = lazy(() => import("@/pages/Settings"));

// Protected product routes (session shell), split out of routes.jsx so that
// file stays within its ratchet — same pattern as publicRoutes.jsx.
export const protectedChildren = [
  { path: "dashboard", element: <Dashboard /> },
  { path: "invoices", element: <Invoices /> },
  { path: "templates", element: <InvoiceTemplates /> },
  { path: "invoices/new", element: <InvoiceEditor /> },
  { path: "invoices/:id", element: <InvoiceDetail /> },
  { path: "invoices/:id/edit", element: <InvoiceEditor /> },
  { path: "clients", element: <Clients /> },
  { path: "clients/:id", element: <ClientDetail /> },
  { path: "expenses", element: <Expenses /> },
  { path: "payments", element: <Payments /> },
  { path: "items", element: <Items /> },
  { path: "reports", element: <Reports /> },
  { path: "settings", element: <Settings /> },
];
