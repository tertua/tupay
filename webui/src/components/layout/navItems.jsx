import {
  LayoutGrid,
  FileText,
  Repeat,
  Users,
  Receipt,
  Wallet,
  Package,
  BarChart3,
} from "lucide-react";

export const NAV = [
  { to: "/dashboard", icon: LayoutGrid, labelKey: "sidebar.dashboard" },
  { to: "/invoices", icon: FileText, labelKey: "sidebar.invoices" },
  { to: "/templates", icon: Repeat, labelKey: "sidebar.templates" },
  { to: "/clients", icon: Users, labelKey: "sidebar.clients" },
  { to: "/expenses", icon: Receipt, labelKey: "sidebar.expenses" },
  { to: "/payments", icon: Wallet, labelKey: "sidebar.payments" },
  { to: "/items", icon: Package, labelKey: "sidebar.items" },
  { to: "/reports", icon: BarChart3, labelKey: "sidebar.reports" },
];
