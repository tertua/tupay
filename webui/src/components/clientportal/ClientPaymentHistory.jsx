import { t } from "@/lib/i18n";
import { formatMoney, formatDate } from "@/lib/utils";

// Payment-history list on the public client portal: date · method · amount.
// Every method arrives relabeled by the backend (no provider name leaks).
export default function ClientPaymentHistory({ payments, currency, lang }) {
  if (!payments || payments.length === 0) {
    return <p className="text-sm text-[var(--ink-muted)] py-2">{t(lang, "public.clientHistoryEmpty")}</p>;
  }
  return (
    <div className="divide-y divide-[var(--border)]">
      {payments.map((p) => (
        <div key={p.id} className="flex items-center justify-between gap-3 py-2.5">
          <div className="text-sm text-[var(--ink)]">
            {formatDate(p.paid_on)}
            <span className="text-[var(--ink-muted)]"> · {p.method}</span>
          </div>
          <div className="text-sm font-semibold text-[var(--ink)] tabular">{formatMoney(p.amount, currency)}</div>
        </div>
      ))}
    </div>
  );
}
