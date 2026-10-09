import { Link } from "react-router-dom";
import { Card, CardTitle } from "@/components/ui/Card";
import { useLang } from "@/context/LangContext";
import { cn } from "@/lib/utils";

// Page-local pieces extracted from InvoiceDetail.jsx to keep the page under its
// file-size baseline: the "mark as" pill and the client summary card. Names are
// prefixed to avoid colliding with the CRM components/clients/ClientCard.jsx.
export function InvoiceStatusButton({ active, onClick, icon: Icon, label, tone }) {
  return (
    <button type="button"
      onClick={onClick}
      className={cn(
        "inline-flex items-center gap-1.5 h-8 px-3 rounded-full text-xs font-semibold border transition-colors",
        active
          ? tone === "success"
            ? "bg-[var(--success)]/12 text-[var(--success)] border-transparent"
            : "bg-[var(--ink)] text-[var(--bg)] border-transparent"
          : "bg-[var(--surface)] text-[var(--ink-muted)] border-[var(--border)] hover:text-[var(--ink)]"
      )}
    >
      <Icon size={13} />
      {label}
    </button>
  );
}

export function InvoiceClientCard({ invoice }) {
  const { t } = useLang();
  if (!invoice.client_id) return null;
  return (
    <Card padding="lg">
      <CardTitle className="mb-3">{t("common.client")}</CardTitle>
      <Link
        to={`/clients/${invoice.client_id}`}
        className="flex items-center gap-3 group"
      >
        <div className="h-10 w-10 rounded-full bg-[var(--accent-soft)] text-[var(--accent-strong)] flex items-center justify-center font-semibold">
          {invoice.client_name?.[0]?.toUpperCase() || "?"}
        </div>
        <div className="min-w-0">
          <div className="text-sm font-semibold text-[var(--ink)] group-hover:text-[var(--accent-strong)] truncate">
            {invoice.client_name}
          </div>
          <div className="text-xs text-[var(--ink-muted)] truncate">
            {invoice.client_email || invoice.client_company || t("invDetail.viewProfile")}
          </div>
        </div>
      </Link>
    </Card>
  );
}
