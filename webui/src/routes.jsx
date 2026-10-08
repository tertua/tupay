import { lazy } from "react";
import { Navigate, createBrowserRouter, useLocation } from "react-router-dom";
import { AppShell } from "@/components/layout/AppShell";
import RouteError from "@/components/ui/RouteError";
import { useAuth } from "@/context/AuthContext";
import { useLang } from "@/context/LangContext";
import { fallbackRoute, publicRoutes } from "@/routes/publicRoutes";

const Dashboard = lazy(() => import("@/pages/Dashboard"));
const Invoices = lazy(() => import("@/pages/Invoices")); const InvoiceTemplates = lazy(() => import("@/pages/InvoiceTemplates"));
const InvoiceEditor = lazy(() => import("@/pages/InvoiceEditor"));
const InvoiceDetail = lazy(() => import("@/pages/InvoiceDetail"));
const Clients = lazy(() => import("@/pages/Clients"));
const ClientDetail = lazy(() => import("@/pages/ClientDetail"));
const Expenses = lazy(() => import("@/pages/Expenses"));
const Payments = lazy(() => import("@/pages/Payments"));
const Items = lazy(() => import("@/pages/Items"));
const Reports = lazy(() => import("@/pages/Reports"));
const Settings = lazy(() => import("@/pages/Settings"));

function ProtectedShell() {
  const { user, loading } = useAuth();
  const { t } = useLang();
  const location = useLocation();
  if (loading) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-[var(--bg)] text-[var(--ink-muted)] text-sm">
        {t("common.loading")}
      </div>
    );
  }
  if (!user) return <Navigate to="/login" state={{ from: location }} replace />;
  return <AppShell />;
}

export const router = createBrowserRouter([
  ...publicRoutes,
  {
    path: "/",
    element: <ProtectedShell />,
    errorElement: <RouteError />,
    children: [
      { path: "dashboard", element: <Dashboard /> },
      { path: "invoices", element: <Invoices /> }, { path: "templates", element: <InvoiceTemplates /> },
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
    ],
  },
  fallbackRoute,
]);
