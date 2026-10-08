import { Plus } from "lucide-react";
import { Card } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { StatusBadge } from "@/components/ui/Badge";
import { useLang } from "@/context/LangContext";
import { formatDate, formatMoney, todayDateInput } from "@/lib/utils";

// Presentational pieces split out of ClientDetail.jsx so that page stays
// within its size ratchet.
export function ChartFallback() {
  return <div className="h-[320px] rounded-3xl bg-[var(--surface-2)] animate-pulse" />;
}

// displayStatus / isOverdue mirror the backend effective-status rules for
// rendering; the server is the source of truth for the actual filters.
export function displayStatus(inv) {
  return inv.effective_status || inv.status;
}

export function isOverdue(inv) {
  return displayStatus(inv) === "sent" && inv.due_date && inv.due_date < todayDateInput();
}

export function ContactRow({ icon: Icon, value, href }) {
  if (!value) return null;
  const content = (
    <div className="flex items-center gap-3 text-sm">
      <div className="h-8 w-8 rounded-xl bg-[var(--surface-2)] text-[var(--ink-muted)] flex items-center justify-center shrink-0">
        <Icon size={14} />
      </div>
      <span className="text-[var(--ink)] truncate">{value}</span>
    </div>
  );
  return href ? (
    <a href={href} className="block hover:text-[var(--accent-strong)]">
      {content}
    </a>
  ) : (
    content
  );
}

export function MiniStat({ label, value, warn }) {
  return (
    <Card padding="lg">
      <div className="text-[10px] uppercase tracking-wider text-[var(--ink-muted)] font-semibold">{label}</div>
      <div
        className={`font-display text-2xl font-semibold tabular mt-1.5 truncate ${
          warn ? "text-[var(--warning)]" : "text-[var(--ink)]"
        }`}
      >
        {value}
      </div>
    </Card>
  );
}

// InvoiceHistory renders the client-detail invoice list (optionally filtered
// to overdue by the server) with a status badge per row.
export function InvoiceHistory({ invoices, onOpenInvoice, onNewInvoice }) {
  const { t } = useLang();

  if (invoices.length === 0) {
    return (
      <div className="py-10 text-center">
        <p className="text-sm text-[var(--ink-muted)]">{t("clientDetail.noInvoices")}</p>
        <Button variant="soft" size="sm" className="mt-3" onClick={() => onNewInvoice()}>
          <Plus size={14} /> {t("clientDetail.createOne")}
        </Button>
      </div>
    );
  }

  return (
    <div className="divide-y divide-[var(--border)]">
      {invoices.map((inv) => (
        <button
          type="button"
          key={inv.id}
          onClick={() => onOpenInvoice(inv.id)}
          className="w-full flex items-center gap-3 py-3 text-left hover:opacity-90"
        >
          <div className="flex-1 min-w-0">
            <div className="text-sm font-semibold text-[var(--ink)] tabular">{inv.invoice_number}</div>
            <div className="text-xs text-[var(--ink-muted)]">
              {t("clientDetail.issuedDue", { issued: formatDate(inv.issue_date), due: formatDate(inv.due_date) })}
            </div>
          </div>
          <div className="text-sm font-semibold text-[var(--ink)] tabular shrink-0">
            {formatMoney(inv.total, inv.currency)}
          </div>
          <StatusBadge status={isOverdue(inv) ? "overdue" : displayStatus(inv)} />
        </button>
      ))}
    </div>
  );
}
