import { useNavigate, useParams, Link } from "react-router-dom";
import {
  ArrowLeft,
  Pencil,
  Trash2,
  Loader2,
  Send,
  Undo2,
  Mail,
} from "lucide-react";
import { Card, CardTitle } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { StatusBadge } from "@/components/ui/Badge";
import { EmptyState } from "@/components/ui/EmptyState";
import { InvoicePdfDownload } from "@/components/invoice/InvoicePdfDownload";
import { InvoicePreview } from "@/components/invoice/InvoicePreview";
import { InvoicePaymentCard } from "@/components/invoice/InvoicePaymentCard";
import { InvoiceReminderCard } from "@/components/invoice/InvoiceReminderCard";
import { InvoiceStatusActions } from "@/components/invoice/InvoiceStatusActions";
import {
  useInvoice,
  useSetInvoiceStatus,
  useDeleteInvoice,
} from "@/hooks/useInvoices";
import { useSettings } from "@/hooks/useSettings";
import { useLang } from "@/context/LangContext";
import { formatMoney, cn } from "@/lib/utils";

export default function InvoiceDetail() {
  const { id } = useParams();
  const nav = useNavigate();
  const { t, lang } = useLang();
  const { data: invoice, isLoading, error } = useInvoice(id);
  const { data: settings } = useSettings();
  const setStatus = useSetInvoiceStatus();
  const del = useDeleteInvoice();

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-24 text-[var(--ink-muted)]">
        <Loader2 className="animate-spin" size={20} />
      </div>
    );
  }
  if (error?.status === 401) return null;
  if (error || !invoice) {
    return <EmptyState icon={Mail} title={t("invDetail.notFound")} description={t("invDetail.deleted")} />;
  }

  const st = invoice.effective_status;
  const isPaid = st === "paid";
  const total = Number(invoice.total) || 0;
  const paidAmount = Number(invoice.paid_amount) || 0;
  // Money-paid (real cash covering total) is fully locked; a manually-marked
  // paid invoice with no money can still be reopened via status.
  const isMoneyPaid = isPaid && total > 0 && paidAmount >= total;
  const canEdit = !isPaid && st !== "pending";
  const canDelete = !isPaid && st !== "pending";
  const lockStatus = isMoneyPaid;

  async function onDelete() {
    if (!window.confirm(t("invDetail.confirmDelete", { number: invoice.invoice_number }))) return;
    try {
      await del.mutateAsync(id);
      nav("/invoices");
    } catch {
      // Backend rejects paid/pending edits+deletes (422); the banner above explains.
    }
  }

  return (
    <div className="max-w-[1100px]">
      {/* header */}
      <div className="flex items-center justify-between gap-4 mb-6 flex-wrap">
        <div className="flex items-center gap-3">
          <button type="button"
            onClick={() => nav("/invoices")}
            className="h-9 w-9 rounded-full flex items-center justify-center border border-[var(--border)] bg-[var(--surface)] text-[var(--ink-muted)] hover:text-[var(--ink)] shadow-card"
          >
            <ArrowLeft size={16} />
          </button>
          <div>
            <div className="flex items-center gap-3">
              <h2 className="font-display text-2xl font-semibold tracking-tight">
                {invoice.invoice_number}
              </h2>
              <StatusBadge status={st} />
            </div>
            <p className="text-sm text-[var(--ink-muted)]">
              {invoice.client_name || t("common.noClient")} · {formatMoney(invoice.total, invoice.currency)}
            </p>
          </div>
        </div>

        <div className="flex items-center gap-2 flex-wrap">
          <InvoicePdfDownload invoice={invoice} settings={settings} lang={lang} label="PDF" />
          {canEdit && (
          <Button variant="outline" onClick={() => nav(`/invoices/${id}/edit`)}>
            <Pencil size={15} /> {t("common.edit")}
          </Button>
          )}
          {canDelete ? (
          <Button
            variant="ghost"
            onClick={onDelete}
            aria-label={t("common.delete")}
            title={t("common.delete")}
            className="text-[var(--danger)] hover:bg-[var(--danger)]/10"
          >
            <Trash2 size={15} />
          </Button>
          ) : null}
        </div>
      </div>

      {isPaid || (st === "pending" && invoice.status !== "pending") ? (
        <div className="mb-6 rounded-2xl border border-[var(--border)] bg-[var(--surface)] px-4 py-3">
          <div className="text-xs font-semibold text-[var(--ink)]">{st === "pending" && invoice.status !== "pending" ? t("status.pending") : t("invDetail.paidLocked")}</div>
          <p className="text-xs text-[var(--ink-muted)] mt-0.5">
            {st === "pending" && invoice.status !== "pending" ? t("payments.onlineActive") : isMoneyPaid ? t("invDetail.paidLockedDesc") : t("invDetail.manuallyPaidDesc")}
          </p>
        </div>
      ) : null}

      <InvoiceStatusActions invoice={invoice} />

      {/* status controls (hidden while paid-locked or money in flight) */}
      {lockStatus || st === "pending" ? null : (
      <div className="flex items-center gap-2 mb-6 flex-wrap">
        <span className="text-xs text-[var(--ink-muted)] mr-1">{t("invDetail.markAs")}</span>
        <StatusButton
          active={invoice.status === "draft"}
          onClick={() => { if (window.confirm(t("invDetail.confirmMarkAs", { number: invoice.invoice_number, status: t("status.draft") }))) setStatus.mutate({ id, status: "draft" }); }}
          icon={Undo2}
          label={t("status.draft")}
        />
        {invoice.client_id ? <StatusButton
          active={invoice.status === "sent"}
          onClick={() => { if (window.confirm(t("invDetail.confirmMarkAs", { number: invoice.invoice_number, status: t("status.sent") }))) setStatus.mutate({ id, status: "sent" }); }}
          icon={Send}
          label={t("status.sent")}
        /> : null}
        {setStatus.isPending && <Loader2 size={14} className="animate-spin text-[var(--ink-muted)]" />}
      </div>
      )}

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-5">
        {/* Invoice preview */}
        <div className="lg:col-span-2">
          <InvoicePreview invoice={invoice} settings={settings} />
        </div>

        {/* Side: payment + reminder + client */}
        <div className="space-y-5">
          <InvoicePaymentCard invoice={invoice} />
          {!isPaid && st !== "pending" && <InvoiceReminderCard invoiceId={id} />}
          <ClientCard invoice={invoice} />
        </div>
      </div>
    </div>
  );
}

function StatusButton({ active, onClick, icon: Icon, label, tone }) {
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

function ClientCard({ invoice }) {
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
