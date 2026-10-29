import { NavLink } from "react-router-dom";
import { Settings, ShieldCheck } from "lucide-react";
import { cn } from "@/lib/utils";
import { useAuth } from "@/context/AuthContext";
import { useLang } from "@/context/LangContext";
import { useAppName } from "@/hooks/useConfig";
import AILogo from "./AILogo";
import { UserCard } from "./SidebarUserCard";
import { NAV } from "./navItems";
import { MobileNav } from "./MobileNav";

const ROW_BASE =
  "relative flex items-center h-11 w-11 rounded-2xl overflow-hidden " +
  "group-hover/sidebar:w-[200px] " +
  "transition-[width,background-color,color,box-shadow] duration-300 ease-[cubic-bezier(0.16,1,0.3,1)]";

const LABEL_BASE =
  "text-sm font-medium whitespace-nowrap pr-4 " +
  "opacity-0 -translate-x-1 " +
  "transition-[opacity,transform] duration-200 ease-out " +
  "group-hover/sidebar:opacity-100 group-hover/sidebar:translate-x-0 group-hover/sidebar:delay-100";

// href = cross-entry link (the admin console lives in admin.html, so it is a full page load, not a NavLink transition).
function NavItem({ to, href, icon: Icon, labelKey }) {
  const { t } = useLang();
  const label = t(labelKey);
  const row = (isActive) => (
    <div
      className={cn(
        ROW_BASE,
        isActive
          ? "bg-[var(--accent-strong)] text-[var(--bg)] shadow-card"
          : "text-[var(--ink-muted)] hover:bg-[var(--surface-2)] hover:text-[var(--ink)]",
      )}
    >
      <span className="h-11 w-11 flex items-center justify-center shrink-0">
        <Icon size={18} strokeWidth={2} />
      </span>
      <span className={LABEL_BASE}>{label}</span>
    </div>
  );
  if (href) return <a href={href} title={label} className="block">{row(false)}</a>;
  return <NavLink to={to} title={label} className="block">{({ isActive }) => row(isActive)}</NavLink>;
}

function ActionRow({ icon: Icon, label, onClick, to }) {
  const inner = (isActive) => (
    <div
      className={cn(
        ROW_BASE,
        isActive
          ? "bg-[var(--accent-strong)] text-[var(--bg)] shadow-card"
          : "text-[var(--ink-muted)] hover:bg-[var(--surface-2)] hover:text-[var(--ink)]"
      )}
    >
      <span className="h-11 w-11 flex items-center justify-center shrink-0">
        <Icon size={18} />
      </span>
      <span className={LABEL_BASE}>{label}</span>
    </div>
  );

  if (to) {
    return (
      <NavLink to={to} title={label} className="block">
        {({ isActive }) => inner(isActive)}
      </NavLink>
    );
  }

  return (
    <button type="button" onClick={onClick} title={label} className="block">
      {inner(false)}
    </button>
  );
}

export function Sidebar() {
  const { user, logout } = useAuth();
  const { t } = useLang();
  const appName = useAppName();

  return (
    <>
    <aside
      className={cn(
        "group/sidebar hidden md:flex shrink-0 h-[calc(100vh-32px)] sticky top-4 ml-4",
        "flex-col items-center justify-between py-5 rounded-3xl",
        "bg-[var(--surface)] border border-[var(--border)] shadow-card overflow-hidden",
        "w-[88px] hover:w-[248px]",
        "transition-[width] duration-300 ease-[cubic-bezier(0.16,1,0.3,1)]",
      )}
    >
      <div className="flex flex-col items-center gap-6 w-full">
        <div
          className={cn(
            "flex items-center h-14 w-14 group-hover/sidebar:w-[200px]",
            "transition-[width] duration-300 ease-[cubic-bezier(0.16,1,0.3,1)]"
          )}
        >
          <div className="h-12 w-12 flex items-center justify-center shrink-0">
            <AILogo label={appName} />
          </div>
          <span
            className={cn(
              "ml-2 font-display text-base font-semibold text-[var(--ink)] whitespace-nowrap",
              "opacity-0 -translate-x-1",
              "transition-[opacity,transform] duration-200 ease-out",
              "group-hover/sidebar:opacity-100 group-hover/sidebar:translate-x-0 group-hover/sidebar:delay-100",
            )}
          >
            {appName}
          </span>
        </div>

        <nav className="flex flex-col items-center gap-1.5">
          {NAV.map((item) => (
            <NavItem key={item.to} {...item} />
          ))}
        </nav>
      </div>

      <div className="flex flex-col items-center gap-2 w-full">
        {user?.role === "admin" && (
          <NavItem href="/admin" icon={ShieldCheck} labelKey="admin.console" />
        )}
        <ActionRow icon={Settings} label={t("sidebar.settings")} to="/settings" />

        <UserCard user={user} logout={logout} t={t} />
      </div>
    </aside>
    <MobileNav />
    </>
  );
}
