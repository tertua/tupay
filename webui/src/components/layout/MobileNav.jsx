import { NavLink } from "react-router-dom";
import { Settings } from "lucide-react";
import { cn } from "@/lib/utils";
import { useLang } from "@/context/LangContext";
import { NAV } from "./navItems";

function MobileTab({ to, icon: Icon, labelKey }) {
  const { t } = useLang();
  const label = t(labelKey);
  return (
    <NavLink
      to={to}
      title={label}
      className={({ isActive }) =>
        cn(
          "flex min-w-[3rem] flex-1 flex-col items-center gap-1 rounded-2xl px-1 py-2",
          isActive
            ? "bg-[var(--accent-strong)] text-[var(--bg)] shadow-card"
            : "text-[var(--ink-muted)]",
        )
      }
    >
      <Icon size={18} strokeWidth={2} />
      <span className="max-w-full truncate text-[10px] font-medium leading-none">{label}</span>
    </NavLink>
  );
}

export function MobileNav() {
  return (
    <>
      <div className="h-24 md:hidden" aria-hidden="true" />
      <nav className="fixed inset-x-3 z-40 md:hidden bottom-[max(0.75rem,env(safe-area-inset-bottom))]">
        <div className="flex items-stretch gap-1 overflow-x-auto rounded-3xl border border-[var(--border)] bg-[var(--surface)]/95 px-2 py-2 shadow-card backdrop-blur">
          {NAV.map((item) => (
            <MobileTab key={item.to} {...item} />
          ))}
          <MobileTab to="/settings" icon={Settings} labelKey="sidebar.settings" />
        </div>
      </nav>
    </>
  );
}
