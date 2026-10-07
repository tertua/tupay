import { useState } from "react";
import { Link } from "react-router-dom";
import { FileDown } from "lucide-react";
import { Badge, INVOICE_STATUS } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { InvoicePdfDownload } from "@/components/invoice/InvoicePdfDownload";
import { usePublicClientInvoice } from "@/hooks/usePublicClient";
import { t } from "@/lib/i18n";
import { formatMoney, formatDate } from "@/lib/utils";

// One invoice row on the public client portal: number, status pill, due date,
// totals, a per-invoice PDF download (detail fetched on demand) and a Pay
// button that routes into the existing /pay/:token flow — never a parallel
// payment path.
export default function ClientInvoiceRow({ invoice, branding, lang, token }) {
  const [wantPdf, setWantPdf] = useState(false);
  const cur = invoice.currency || "IDR";
  const status = INVOICE_STATUS[invoice.effective_status] ? invoice.effective_status : "draft";
  const meta = INVOICE_STATUS[status];
  const canPay = invoice.public_pay_token && status !== "paid" && status !== "pending";

  return (
    <div className="flex flex-col sm:flex-row sm:items-center gap-3 py-4">
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2">
          <span className="text-sm font-semibold text-[var(--ink)] tabular truncate">{invoice.invoice_number}</span>
          <Badge tone={meta.tone}>{t(lang, meta.labelKey)}</Badge>
        </div>
        {invoice.due_date && (
          <div className="text-xs text-[var(--ink-muted)] mt-1">
            {t(lang, "public.clientDue")} {formatDate(invoice.due_date)}
          </div>
        )}
      </div>

      <div className="text-right shrink-0">
        <div className="text-sm font-semibold text-[var(--ink)] tabular">{formatMoney(invoice.total, cur)}</div>
        <div className="text-xs text-[var(--ink-muted)] tabular">
          {t(lang, "public.clientBalance")} {formatMoney(invoice.balance, cur)}
        </div>
      </div>

      <div className="flex items-center gap-2 shrink-0">
        {canPay ? (
          <Link to={`/pay/${invoice.public_pay_token}`}>
            <Button variant="accent" size="sm">{t(lang, "public.clientPay")}</Button>
          </Link>
        ) : status !== "paid" ? (
          <span className="text-[11px] text-[var(--ink-muted)] max-w-[140px] text-right">
            {t(lang, "public.clientNoOnline")}
          </span>
        ) : null}
        <InvoicePdfButton
          token={token}
          invoiceID={invoice.id}
          branding={branding}
          lang={lang}
          wantPdf={wantPdf}
          onActivate={() => setWantPdf(true)}
        />
      </div>
    </div>
  );
}

// InvoicePdfButton keeps the pending/inactive states in one place: until the
// payer asks for the PDF the per-invoice detail is never fetched, and once it
// resolves the existing InvoicePdfDownload boundary renders the file.
function InvoicePdfButton({ token, invoiceID, branding, lang, wantPdf, onActivate }) {
  const { data, isLoading } = usePublicClientInvoice(token, invoiceID, wantPdf);
  const label = t(lang, "public.clientDownloadPdf");

  if (data) {
    return <InvoicePdfDownload invoice={data} settings={branding} lang={lang} publicView label={label} />;
  }
  return (
    <Button variant="outline" size="sm" onClick={onActivate} disabled={isLoading}>
      <FileDown size={14} /> {label}
    </Button>
  );
}
