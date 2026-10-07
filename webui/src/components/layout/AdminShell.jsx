import { useEffect } from "react";
import { NavLink, Outlet } from "react-router-dom";
import { ArrowLeft, ShieldCheck } from "lucide-react";
import { cn } from "@/lib/utils";
import { useLang } from "@/context/LangContext";
import { useAppName } from "@/hooks/useConfig";
import { AdminHeaderStrip } from "@/components/admin/AdminHeaderStrip";

// Selected section reads as a bottom-underline accent (not a colored pill), so
// the tab row sits inline in the header instead of on a decorative wrapper.
function AdminTab({ to, children }) {
  return (
    <NavLink
      to={to}
      className={({ isActive }) =>
        cn(
          "relative -mb-px border-b-[3px] px-3 py-4 text-sm font-medium whitespace-nowrap transition-colors duration-200",
          isActive
            ? "border-[var(--accent)] text-[var(--accent-strong)]"
            : "border-transparent text-[var(--ink-muted)] hover:border-[var(--border)] hover:text-[var(--ink)]",
        )
      }
    >
      {children}
    </NavLink>
  );
}

export function AdminShell() {
  const { t } = useLang();
  const appName = useAppName();
  useEffect(() => {
    document.title = `${appName} — ${t("admin.console")}`;
  }, [appName, t]);
  return (
    <div className="min-h-screen flex flex-col bg-[var(--bg)]">
      <header className="sticky top-0 z-30 border-b border-[var(--border)] bg-[var(--surface)]/80 backdrop-blur-xl">
        <div className="mx-auto max-w-[1600px] px-4 sm:px-6 lg:px-8">
          <div className="flex h-14 items-center justify-between gap-4">
            <div className="flex items-center gap-3 min-w-0">
              <div className="flex items-center gap-2.5">
                <div className="flex h-7 w-7 items-center justify-center rounded-lg bg-[var(--accent)]/10">
                  <ShieldCheck size={16} className="text-[var(--accent)]" />
                </div>
                <span className="text-base font-semibold text-[var(--ink)]">
                  {t("admin.console")}
                </span>
              </div>
            </div>
            <nav className="hidden self-stretch md:flex items-center gap-1">
              <AdminTab to="/gateway">{t("admin.gateway")}</AdminTab>
              <AdminTab to="/settlement">{t("gateway.settlement")}</AdminTab>
              <AdminTab to="/users">{t("admin.users")}</AdminTab>
            </nav>
            <div className="flex items-center">
              <a
                href="/dashboard"
                className="flex items-center gap-2 rounded-lg px-2.5 py-1.5 text-sm font-medium text-[var(--ink-muted)] outline-none transition-colors duration-200 hover:bg-[var(--surface-2)] hover:text-[var(--ink)] focus-visible:ring-2 focus-visible:ring-[var(--accent)]/30"
              >
                <ArrowLeft size={16} />
                <span className="hidden sm:inline">{t("admin.backToApp")}</span>
              </a>
            </div>
          </div>
          <nav className="md:hidden -mb-px flex items-center gap-1 overflow-x-auto scrollbar-hide">
            <AdminTab to="/gateway">{t("admin.gateway")}</AdminTab>
            <AdminTab to="/settlement">{t("gateway.settlement")}</AdminTab>
            <AdminTab to="/users">{t("admin.users")}</AdminTab>
          </nav>
          <div className="hidden md:block">
            <AdminHeaderStrip />
          </div>
        </div>
      </header>
      <main className="flex-1 px-4 sm:px-6 lg:px-8 py-8 max-w-[1600px] mx-auto w-full">
        <Outlet />
      </main>
    </div>
  );
}
