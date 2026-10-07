import { useEffect, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import {
  Loader2,
  Trash2,
  Copy,
  Check,
  Mail,
  Plus,
  Wallet,
  Link2,
  ExternalLink,
} from "lucide-react";
import { Card, CardTitle } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { useLang } from "@/context/LangContext";
import { formatMoney, formatDate } from "@/lib/utils";
import { paymentMethodLabel } from "@/lib/paymentLabels";
import { usePaymentMutations } from "@/hooks/usePayments";
import { invoiceKey } from "@/hooks/useInvoices";
import { RecordPaymentModal } from "@/components/payments/RecordPaymentModal";
import { VoidPaymentModal } from "@/components/payments/VoidPaymentModal";
import { paymentsApi } from "@/api/payments";
import { TotalLine } from "@/components/invoice/InvoicePreview";

export function InvoicePaymentCard({ invoice }) {
  const { t } = useLang();
  const qc = useQueryClient();
  const { remove } = usePaymentMutations();
  const [modalOpen, setModalOpen] = useState(false);
  const currency = invoice.currency;
  const total = Number(invoice.total) || 0;
  const paid = Number(invoice.paid_amount) || 0;
  const balance = Number(invoice.balance) || 0;
  const pct = total > 0 ? Math.min(100, (paid / total) * 100) : 0;
  const payments = invoice.payments || [];
  const isPaid = invoice.effective_status === "paid";

  // Payment link is persisted on the invoice (BE returns payment_link when
  // one exists), so it survives reloads. POST /payments/online stays
  // get-or-create: it returns the existing link when already created.
  const [shareLink, setShareLink] = useState(() => invoice.payment_link || null);
  const [shareLoading, setShareLoading] = useState(false);
  const [shareErr, setShareErr] = useState("");
  const [linkCopied, setLinkCopied] = useState(false);
  const [voidTarget, setVoidTarget] = useState(null);
  const [email, setEmail] = useState(invoice.client_email || "");
  const [sending, setSending] = useState(false);
  const [sent, setSent] = useState(false);
  const [sendErr, setSendErr] = useState("");
  const canShareOnline = balance > 0 && invoice.effective_status !== "draft" && invoice.effective_status !== "pending" && currency === "IDR";
  const showIdrHint = balance > 0 && invoice.effective_status !== "draft" && currency !== "IDR" && !shareLink;
  const shareUrl = shareLink ? new URL(shareLink.url, window.location.origin).href : "";

  useEffect(() => {
    if (invoice.payment_link) setShareLink(invoice.payment_link);
  }, [invoice.payment_link]);
  useEffect(() => {
    setEmail(invoice.client_email || "");
  }, [invoice.client_email]);

  async function onVoid(reason) {
    if (!voidTarget) return;
    await remove.mutateAsync({ id: voidTarget.id, reason });
    setVoidTarget(null);
  }

  async function onShare() {
    if (shareLink || shareLoading) return;
    setShareLoading(true);
    setShareErr("");
    try {
      const res = await paymentsApi.createOnlineLink(invoice.id);
      setShareLink(res);
      qc.invalidateQueries({ queryKey: invoiceKey(invoice.id) });
    } catch (e) {
      if (e.status !== 401) setShareErr(e.message || t("payments.saveFailed"));
    } finally {
      setShareLoading(false);
    }
  }

  async function onSendLink() {
    if (sending || !email.trim() || !shareLink) return;
    setSending(true);
    setSendErr("");
    setSent(false);
    try {
      await paymentsApi.sendOnlineLink(invoice.id, email.trim());
      setSent(true);
    } catch (e) {
      if (e.status !== 401) setSendErr(e.message || t("payments.saveFailed"));
    } finally {
      setSending(false);
    }
  }

  async function copyShareLink() {
    try {
      await navigator.clipboard.writeText(shareUrl);
      setLinkCopied(true);
      setTimeout(() => setLinkCopied(false), 1500);
    } catch {
      /* clipboard unavailable */
    }
  }

  return (
    <Card padding="lg">
      <div className="flex items-center justify-between mb-3">
        <div className="flex items-center gap-2">
          <div className="h-8 w-8 rounded-xl bg-[var(--accent-soft)] text-[var(--accent-strong)] flex items-center justify-center">
            <Wallet size={15} />
          </div>
          <CardTitle>{t("invDetail.payments")}</CardTitle>
        </div>
        <div className="flex items-center gap-2">
          {balance > 0 && invoice.effective_status !== "pending" && !(invoice.payment_method === "Online" && (shareLink || canShareOnline)) && (
            <Button variant="accent" size="iconSm" onClick={() => setModalOpen(true)} aria-label={t("invDetail.recordPayment")} title={t("invDetail.recordPayment")}><Plus size={15} /></Button>
          )}
        </div>
      </div>

      <div className="space-y-1.5 text-sm mb-3">
        <TotalLine label={t("common.total")} value={formatMoney(total, currency)} />
        <div className="flex items-center justify-between">
          <span className="text-[var(--ink-muted)]">{t("invDetail.paidAmount")}</span>
          <span className="tabular font-semibold text-[var(--success)]">{formatMoney(paid, currency)}</span>
        </div>
        <div className="flex items-center justify-between">
          <span className="text-[var(--ink-muted)]">{t("invDetail.balanceDue")}</span>
          <span className={`tabular font-semibold ${balance > 0 ? "text-[var(--danger)]" : "text-[var(--ink)]"}`}>
            {formatMoney(balance, currency)}
          </span>
        </div>
      </div>

      <div className="h-1.5 w-full rounded-full bg-[var(--surface-2)] mb-4 overflow-hidden">
        <div className="h-full rounded-full bg-[var(--success)] transition-all" style={{ width: `${pct}%` }} />
      </div>

      {canShareOnline && !shareLink && (
        <Button variant="outline" size="sm" className="w-full mb-3" onClick={onShare} disabled={shareLoading}>
          {shareLoading ? <Loader2 size={13} className="animate-spin" /> : <Link2 size={13} />}
          {t("payments.shareLink")}
        </Button>
      )}
      {showIdrHint && (
        <p className="text-xs text-[var(--ink-muted)] mb-3">{t("payments.onlineOnlyIdr")}</p>
      )}
      {shareErr && !shareLink && <p className="text-xs text-[var(--danger)] mb-3">{shareErr}</p>}
      {shareLink && (
        <div className="rounded-2xl border border-[var(--border)] bg-[var(--surface-2)] px-3 py-2.5 mb-4">
          <div className="flex items-center justify-between gap-2 mb-1.5">
            <span className="text-[11px] font-semibold text-[var(--ink-muted)]">{t("payments.onlineTitle")}</span>
            <span className={`text-[10px] font-semibold px-2 py-0.5 rounded-full ${isPaid ? "bg-[var(--ink-muted)]/15 text-[var(--ink-muted)]" : "bg-[var(--success)]/12 text-[var(--success)]"}`}>
              {isPaid ? t("payments.onlineInactive") : t("payments.onlineActive")}
            </span>
          </div>
          <div className="flex items-center gap-3">
          <span className="flex-1 min-w-0 text-xs text-[var(--ink)] truncate">{shareUrl}</span>
          <button type="button" onClick={copyShareLink} aria-label={t("payments.onlineCopy")} title={linkCopied ? t("payments.onlineCopied") : t("payments.onlineCopy")}
            className="shrink-0 inline-flex items-center p-1 text-[var(--accent-strong)]">
            {linkCopied ? <Check size={13} /> : <Copy size={13} />}
          </button>
          <a href={shareUrl} target="_blank" rel="noreferrer" aria-label={t("payments.onlineOpen")}
            className="shrink-0 inline-flex items-center p-1 text-[var(--ink-muted)] hover:text-[var(--ink)]">
            <ExternalLink size={13} />
          </a>
          </div>
          {!isPaid && (
          <div className="mt-2 pt-2 border-t border-[var(--border)]">
          <div className="flex items-center gap-3">
              <input
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                placeholder={t("payments.onlineEmailPlaceholder")}
                className="flex-1 min-w-0 h-8 rounded-full border border-[var(--border)] bg-[var(--surface)] px-3 text-xs text-[var(--ink)] outline-none focus:border-[var(--accent)]/50"
              />
              <Button variant="soft" size="sm" onClick={onSendLink} disabled={sending || !email.trim()}>
                {sending ? <Loader2 size={12} className="animate-spin" /> : <Mail size={12} />}
                {sending ? t("payments.onlineSending") : t("payments.onlineSend")}
              </Button>
            </div>
            {sent && <p className="text-[11px] text-[var(--success)] mt-1.5">{t("payments.onlineSent")}</p>}
            {sendErr && <p className="text-[11px] text-[var(--danger)] mt-1.5">{sendErr}</p>}
          </div>
          )}
        </div>
      )}

      {payments.length === 0 ? (
        <p className="text-xs text-[var(--ink-muted)]">{t("invDetail.noPayments")}</p>
      ) : (
        <ul className="divide-y divide-[var(--border)]">
          {payments.map((p) => (
            <li key={p.id} className="flex items-center gap-2 py-2.5 group">
              <div className="min-w-0 flex-1">
                <div className="text-xs font-semibold text-[var(--ink)]">{formatDate(p.paid_on)}</div>
                <div className="text-[11px] text-[var(--ink-muted)]">{paymentMethodLabel(p.method) || "—"}{p.txn_id ? ` · ${p.txn_id}` : ""}</div>
              </div>
              <div className="text-sm font-semibold text-[var(--success)] tabular">{formatMoney(p.amount, currency)}</div>
              {p.can_void !== false ? (
                <button type="button"
                  onClick={() => setVoidTarget(p)}
                  className="md:opacity-0 md:group-hover:opacity-100 md:group-focus-within:opacity-100 focus-visible:opacity-100 transition-opacity h-6 w-6 rounded-full flex items-center justify-center text-[var(--ink-muted)] hover:bg-[var(--surface-2)] hover:text-[var(--danger)]"
                  aria-label={t("payments.voidTitle")}
                >
                  <Trash2 size={12} />
                </button>
              ) : (
                <span aria-hidden="true" className="h-6 w-6" />
              )}
            </li>
          ))}
        </ul>
      )}

      <RecordPaymentModal
        open={modalOpen}
        onClose={() => setModalOpen(false)}
        invoiceId={invoice.id}
        invoiceNumber={invoice.invoice_number}
        amount={balance}
      />
      <VoidPaymentModal
        open={!!voidTarget}
        onClose={() => setVoidTarget(null)}
        payment={voidTarget}
        currency={currency}
        onVoid={onVoid}
      />
    </Card>
  );
}
