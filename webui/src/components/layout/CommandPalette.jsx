import { useEffect, useMemo, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { AnimatePresence, motion } from "framer-motion";
import {
  Search,
  LayoutGrid,
  FileText,
  Users,
  Settings as SettingsIcon,
  Plus,
  CornerDownLeft,
} from "lucide-react";
import { cn, formatMoney } from "@/lib/utils";
import { useLang } from "@/context/LangContext";
import { useInvoices } from "@/hooks/useInvoices";
import { useClients } from "@/hooks/useClients";

function getNavItems(t) {
  return [
    { id: "nav:dashboard", kind: "nav", label: t("palette.dashboard"), hint: t("palette.dashboardHint"), to: "/dashboard", icon: LayoutGrid },
    { id: "nav:invoices", kind: "nav", label: t("palette.invoices"), hint: t("palette.invoicesHint"), to: "/invoices", icon: FileText },
    { id: "nav:new", kind: "nav", label: t("palette.newInvoice"), hint: t("palette.newInvoiceHint"), to: "/invoices/new", icon: Plus },
    { id: "nav:clients", kind: "nav", label: t("palette.clients"), hint: t("palette.clientsHint"), to: "/clients", icon: Users },
    { id: "nav:settings", kind: "nav", label: t("palette.settings"), hint: t("palette.settingsHint"), to: "/settings", icon: SettingsIcon },
  ];
}

function scoreMatch(query, text) {
  if (!query) return 1;
  const q = query.toLowerCase();
  const t = (text || "").toLowerCase();
  if (!t) return 0;
  if (t.startsWith(q)) return 3;
  if (t.includes(q)) return 2;
  let qi = 0;
  for (let i = 0; i < t.length && qi < q.length; i++) {
    if (t[i] === q[qi]) qi++;
  }
  return qi === q.length ? 1 : 0;
}

export function CommandPalette({ open, onClose }) {
  const navigate = useNavigate();
  const { t } = useLang();
  const [query, setQuery] = useState("");
  const [activeIdx, setActiveIdx] = useState(0);
  const inputRef = useRef(null);
  const listRef = useRef(null);

  const { data: invoices } = useInvoices();
  const { data: clientsData } = useClients();

  useEffect(() => {
    if (open) {
      setQuery("");
      setActiveIdx(0);
      const tmr = setTimeout(() => inputRef.current?.focus(), 30);
      return () => clearTimeout(tmr);
    }
  }, [open]);

  const items = useMemo(() => {
    const NAV_ITEMS = getNavItems(t);
    const invoiceItems = (invoices || []).map((i) => ({
      id: `invoice:${i.id}`,
      kind: "invoice",
      label: `${i.invoice_number} · ${i.client_name || t("common.noClient")}`,
      hint: `${t("status." + (i.effective_status || "draft"))} · ${formatMoney(i.total, i.currency)}`,
      to: `/invoices/${i.id}`,
      icon: FileText,
    }));
    const clientItems = (clientsData?.clients || []).map((c) => ({
      id: `client:${c.id}`,
      kind: "client",
      label: c.name,
      hint: c.company || c.email || t("common.client"),
      to: `/clients/${c.id}`,
      icon: Users,
    }));

    const pool = [...NAV_ITEMS, ...invoiceItems, ...clientItems];
    if (!query.trim()) return pool;

    return pool
      .map((it) => ({ it, score: scoreMatch(query.trim(), `${it.label} ${it.hint || ""}`) }))
      .filter((x) => x.score > 0)
      .sort((a, b) => b.score - a.score)
      .map((x) => x.it);
  }, [invoices, clientsData, query, t]);

  useEffect(() => setActiveIdx(0), [query]);

  useEffect(() => {
    const el = listRef.current?.querySelector(`[data-idx="${activeIdx}"]`);
    el?.scrollIntoView({ block: "nearest" });
  }, [activeIdx]);

  function handleKeyDown(e) {
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setActiveIdx((i) => Math.min(items.length - 1, i + 1));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setActiveIdx((i) => Math.max(0, i - 1));
    } else if (e.key === "Enter") {
      e.preventDefault();
      const it = items[activeIdx];
      if (it) {
        navigate(it.to);
        onClose();
      }
    } else if (e.key === "Escape") {
      e.preventDefault();
      onClose();
    }
  }

  const groups = [
    { key: "nav", title: t("palette.navigate"), items: items.filter((i) => i.kind === "nav") },
    { key: "invoice", title: t("palette.invoices"), items: items.filter((i) => i.kind === "invoice") },
    { key: "client", title: t("palette.clients"), items: items.filter((i) => i.kind === "client") },
  ];

  let renderIdx = -1;
  function renderItem(it) {
    renderIdx += 1;
    const idx = renderIdx;
    const Icon = it.icon;
    const isActive = idx === activeIdx;
    return (
      <button type="button"
        key={it.id}
        data-idx={idx}
        onMouseEnter={() => setActiveIdx(idx)}
        onClick={() => {
          navigate(it.to);
          onClose();
        }}
        className={cn(
          "w-full flex items-center gap-3 px-3 py-2.5 rounded-xl text-left transition-colors",
          isActive
            ? "bg-[var(--accent-soft)] text-[var(--ink)]"
            : "hover:bg-[var(--surface-2)] text-[var(--ink)]"
        )}
      >
        <div
          className={cn(
            "h-9 w-9 rounded-xl flex items-center justify-center shrink-0",
            isActive
              ? "bg-[var(--surface)] text-[var(--accent-strong)]"
              : "bg-[var(--surface-2)] text-[var(--ink-muted)]"
          )}
        >
          <Icon size={16} />
        </div>
        <div className="flex-1 min-w-0">
          <div className="text-sm font-medium truncate">{it.label}</div>
          {it.hint && (
            <div className="text-xs text-[var(--ink-muted)] truncate capitalize">{it.hint}</div>
          )}
        </div>
        {isActive && (
          <span className="text-xs text-[var(--ink-muted)] flex items-center gap-1 shrink-0">
            <CornerDownLeft size={12} /> {t("palette.enter")}
          </span>
        )}
      </button>
    );
  }

  return (
    <AnimatePresence>
      {open && (
        <motion.div
          className="fixed inset-0 z-50 flex items-start justify-center pt-[14vh] px-4"
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          transition={{ duration: 0.15 }}
        >
          <div
            className="absolute inset-0 bg-[var(--ink)]/30 backdrop-blur-sm"
            onClick={onClose}
          />
          <motion.div
            role="dialog"
            aria-label="Command palette"
            initial={{ opacity: 0, y: -8, scale: 0.98 }}
            animate={{ opacity: 1, y: 0, scale: 1 }}
            exit={{ opacity: 0, y: -4, scale: 0.98 }}
            transition={{ duration: 0.18, ease: [0.16, 1, 0.3, 1] }}
            className="relative w-full max-w-[640px] rounded-3xl bg-[var(--surface)] border border-[var(--border)] shadow-hover overflow-hidden"
          >
            <div className="flex items-center gap-3 px-5 h-14 border-b border-[var(--border)]">
              <Search size={16} className="text-[var(--ink-muted)] shrink-0" />
              <input
                ref={inputRef}
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                onKeyDown={handleKeyDown}
                placeholder={t("topbar.searchPlaceholder")}
                className="flex-1 bg-transparent outline-none text-sm text-[var(--ink)] placeholder:text-[var(--ink-muted)]"
              />
              <kbd className="hidden sm:inline-flex items-center gap-1 text-[10px] px-2 h-6 rounded-md bg-[var(--surface-2)] text-[var(--ink-muted)] border border-[var(--border)] font-medium">
                Esc
              </kbd>
            </div>

            <div ref={listRef} className="max-h-[52vh] overflow-y-auto p-2">
              {items.length === 0 && (
                <div className="text-center text-sm text-[var(--ink-muted)] py-10">
                  {t("palette.noMatches", { q: query })}
                </div>
              )}

              {groups.map((g) =>
                g.items.length ? (
                  <div key={g.key} className="mb-1">
                    <div className="px-3 pt-2 pb-1 text-[10px] uppercase tracking-wider text-[var(--ink-muted)] font-semibold">
                      {g.title}
                    </div>
                    <div className="flex flex-col gap-0.5">{g.items.map(renderItem)}</div>
                  </div>
                ) : null
              )}
            </div>

            <div className="flex items-center justify-between px-5 h-10 border-t border-[var(--border)] bg-[var(--surface-2)]/60 text-[11px] text-[var(--ink-muted)]">
              <div className="flex items-center gap-3">
                <span className="flex items-center gap-1">
                  <kbd className="px-1.5 h-5 rounded bg-[var(--surface)] border border-[var(--border)] inline-flex items-center">↑</kbd>
                  <kbd className="px-1.5 h-5 rounded bg-[var(--surface)] border border-[var(--border)] inline-flex items-center">↓</kbd>
                  {t("palette.toNavigate")}
                </span>
                <span className="flex items-center gap-1">
                  <kbd className="px-1.5 h-5 rounded bg-[var(--surface)] border border-[var(--border)] inline-flex items-center">↵</kbd>
                  {t("palette.toSelect")}
                </span>
              </div>
              <span>{items.length === 1 ? t("palette.result", { n: items.length }) : t("palette.results", { n: items.length })}</span>
            </div>
          </motion.div>
        </motion.div>
      )}
    </AnimatePresence>
  );
}
