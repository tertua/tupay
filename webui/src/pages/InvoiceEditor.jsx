import { useEffect, useMemo, useRef, useState } from "react";
import { useNavigate, useParams, useSearchParams } from "react-router-dom";
import Decimal from "decimal.js";
import {
  Plus,
  Trash2,
  ArrowLeft,
  Save,
  Loader2,
  Sparkles,
  Mail,
} from "lucide-react";
import { Card, CardTitle } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { EmptyState } from "@/components/ui/EmptyState";
import { Input } from "@/components/ui/Input";
import { PaymentMethodField } from "@/components/invoice/PaymentMethodField";
import { CatalogPicker, ReceiptScanButton } from "@/components/invoice/InvoiceEditorTools";
import { useClients } from "@/hooks/useClients";
import { useSettings } from "@/hooks/useSettings";
import {
  useInvoice,
  useCreateInvoice,
  useUpdateInvoice,
} from "@/hooks/useInvoices";
import { aiApi, isAiUnavailable, isAiFailure, isAiRateLimited } from "@/api/ai";
import { useLang } from "@/context/LangContext";
import { CURRENCIES, formatMoney, toDateInput, todayDateInput, addDaysDateInput, cn } from "@/lib/utils";

const decimal = (value) => new Decimal(value || 0);
const round = (value) => decimal(value).toDecimalPlaces(4);
const blankItem = () => ({ description: "", quantity: 1, rate: 0 });

function todayISO() {
  return todayDateInput();
}
function plusDays(days) {
  return addDaysDateInput(days);
}

export default function InvoiceEditor() {
  const { id } = useParams();
  const isEdit = !!id;
  const nav = useNavigate();
  const { t } = useLang();
  const [searchParams] = useSearchParams();
  const preselectClient = searchParams.get("client") || "";

  const { data: clients } = useClients();
  const { data: settings } = useSettings();
  const { data: existing, isLoading: loadingInvoice, error: invoiceError } = useInvoice(id);
  const create = useCreateInvoice();
  const update = useUpdateInvoice();

  const [form, setForm] = useState(null);
  const [saving, setSaving] = useState(false);
  const [err, setErr] = useState("");
  const inFlight = useRef(false);

  // Initialize the form from settings or the existing invoice.
  useEffect(() => {
    if (isEdit) {
      if (existing && !form) {
        setForm({
          client_id: existing.client_id || "",
          status: existing.status,
          issue_date: toDateInput(existing.issue_date) || todayISO(),
          due_date: toDateInput(existing.due_date) || "",
          currency: existing.currency,
          tax_rate: Number(existing.tax_rate) || 0,
          discount: existing.discount || 0,
          notes: existing.notes || "",
          terms: existing.terms || "",
          payment_method: existing.payment_method || "Cash",
          items: existing.items?.length
            ? existing.items.map((it) => ({
                description: it.description,
                quantity: it.quantity,
                rate: it.rate,
              }))
            : [blankItem()],
        });
      }
    } else if (!form && settings) {
      setForm({
        client_id: preselectClient,
        status: "draft",
        issue_date: todayISO(),
        due_date: plusDays(30),
        currency: settings.currency || "IDR",
        tax_rate: Number(settings.tax_rate) || 0,
        discount: 0,
        notes: "",
        terms: "",
        payment_method: "Online",
        items: [blankItem()],
      });
    }
  }, [isEdit, existing, settings, form, preselectClient]);

  const totals = useMemo(() => {
    if (!form) return { subtotal: 0, taxAmount: 0, total: 0 };
    const subtotal = form.items.reduce(
      (sum, it) => sum.plus(decimal(it.quantity).times(decimal(it.rate))),
      decimal(0)
    );
    const disc = Decimal.min(round(form.discount), subtotal);
    const base = Decimal.max(round(subtotal.minus(disc)), decimal(0));
    const taxAmount = round(base.times(decimal(form.tax_rate)).div(100));
    return { subtotal, discount: disc, taxAmount, total: round(base.plus(taxAmount)) };
  }, [form]);

  if (isEdit && invoiceError?.status === 401) return null;
  if (isEdit && invoiceError) {
    return <EmptyState icon={Mail} title={t("invDetail.notFound")} description={t("invDetail.deleted")} />;
  }
  if (!form || (isEdit && loadingInvoice)) {
    return (
      <div className="flex items-center justify-center py-24 text-[var(--ink-muted)]">
        <Loader2 className="animate-spin" size={20} />
      </div>
    );
  }
  // Paid/pending invoices are immutable: block the whole editor instead of
  // letting the backend reject the save. Matches the banner on detail.
  if (isEdit && (existing?.effective_status === "paid" || existing?.effective_status === "pending")) {
    const isPending = existing?.effective_status === "pending";
    const moneyPaid = !isPending && decimal(existing.total).greaterThan(0) &&
      decimal(existing.paid_amount).greaterThanOrEqualTo(existing.total);
    return (
      <div className="max-w-[640px]">
        <Card padding="lg" className="text-center">
          <CardTitle className="mb-2">{isPending ? t("status.pending") : t("invDetail.paidLocked")}</CardTitle>
          <p className="text-sm text-[var(--ink-muted)] mb-5">
            {isPending ? (existing?.status === "pending" ? t("approval.pendingDesc") : t("payments.onlineActive")) : moneyPaid ? t("invDetail.paidLockedDesc") : t("invDetail.manuallyPaidDesc")}
          </p>
          <div className="flex items-center justify-center gap-2">
            <Button variant="outline" onClick={() => nav(`/invoices/${id}`)}>
              {t("common.back")}
            </Button>
          </div>
        </Card>
      </div>
    );
  }

  const set = (patch) => setForm((f) => ({ ...f, ...patch }));
  const setItem = (i, patch) =>
    setForm((f) => ({
      ...f,
      items: f.items.map((it, idx) => (idx === i ? { ...it, ...patch } : it)),
    }));
  const addItem = () => setForm((f) => ({ ...f, items: [...f.items, blankItem()] }));
  const addCatalogItem = (it) =>
    setForm((f) => {
      const line = { description: it.name, quantity: 1, rate: it.rate || 0 };
      const items = [...f.items];
      const blankIdx = items.findIndex((x) => !x.description.trim() && !decimal(x.rate).isPositive());
      if (blankIdx >= 0) items[blankIdx] = line;
      else items.push(line);
      return { ...f, items };
    });
  const removeItem = (i) =>
    setForm((f) => ({
      ...f,
      items: f.items.filter((_, idx) => idx !== i).length
        ? f.items.filter((_, idx) => idx !== i)
        : [blankItem()],
    }));

  async function onSave(overrideStatus) {
    const status = overrideStatus || form.status;
    if (status === "sent" && !form.client_id) return setErr(t("api.client is required to send an invoice"));
    if (inFlight.current) return; inFlight.current = true; setErr("");
    const payload = {
      ...form,
      status,
      client_id: form.client_id || null,
      tax_rate: Number(form.tax_rate) || 0,
      discount: decimal(form.discount).toFixed(4),
      due_date: form.due_date || undefined,
      issue_date: form.issue_date || undefined,
      items: form.items
        .filter((it) => it.description.trim() || decimal(it.rate).greaterThan(0))
        .map((it) => ({ description: it.description, quantity: Number(it.quantity) || 0, rate: decimal(it.rate).toFixed(4) })),
    };
    setSaving(true);
    try {
      const inv = isEdit
        ? await update.mutateAsync({ id, payload })
        : await create.mutateAsync(payload);
      nav(`/invoices/${inv.id}`);
    } catch (e) {
      if (e.status !== 401) setErr(e.message || t("invEditor.saveFailed"));
    } finally {
      inFlight.current = false;
      setSaving(false);
    }
  }

  const selectClass =
    "h-10 w-full rounded-full border border-[var(--border)] bg-[var(--surface)] px-4 text-sm text-[var(--ink)] outline-none focus:border-[var(--accent)]/50 focus:ring-2 focus:ring-[var(--accent)]/15";
  const symbol = CURRENCIES.find((c) => c.code === form.currency)?.symbol || "Rp";

  return (
    <div className="max-w-[1100px]">
      <div className="flex items-center justify-between gap-4 mb-6">
        <div className="flex items-center gap-3">
          <button type="button"
            onClick={() => nav(-1)}
            className="h-9 w-9 rounded-full flex items-center justify-center border border-[var(--border)] bg-[var(--surface)] text-[var(--ink-muted)] hover:text-[var(--ink)] shadow-card"
          >
            <ArrowLeft size={16} />
          </button>
          <div>
            <h2 className="font-display text-2xl font-semibold tracking-tight">
              {isEdit ? t("invEditor.editTitle") : t("invEditor.newTitle")}
            </h2>
            <p className="text-sm text-[var(--ink-muted)]">
              {isEdit ? existing?.invoice_number : t("invEditor.autoNumber")}
            </p>
          </div>
        </div>
        <div className="flex items-center gap-2">
          {form.status === "sent" ? (
            <Button variant="accent" onClick={() => onSave()} disabled={saving}>
              {saving ? <Loader2 size={15} className="animate-spin" /> : <Save size={15} />}
              {t("common.save")}
            </Button>
          ) : (
            <>
              <Button variant="outline" onClick={() => onSave("draft")} disabled={saving}>
                {t("invEditor.saveDraft")}
              </Button>
              <Button variant="accent" onClick={() => onSave("sent")} disabled={saving}>
                {saving ? <Loader2 size={15} className="animate-spin" /> : <Save size={15} />}
                {t("invEditor.saveSend")}
              </Button>
            </>
          )}
        </div>
      </div>

      {err && (
        <div className="mb-4 text-sm text-[var(--danger)] bg-[var(--danger)]/10 rounded-2xl px-4 py-3">
          {err}
        </div>
      )}

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-5">
        {/* Left — main details */}
        <div className="lg:col-span-2 space-y-5">
          {/* meta */}
          <Card padding="lg">
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <Field label={t("common.client")} className="sm:col-span-2">
                <select
                  className={selectClass}
                  value={form.client_id}
                  onChange={(e) => set({ client_id: e.target.value, payment_method: e.target.value ? form.payment_method : "Cash" })}
                >
                  <option value="">{t("invEditor.noClient")}</option>
                  {(clients || []).map((c) => (
                    <option key={c.id} value={c.id}>
                      {c.name}
                      {c.company ? ` (${c.company})` : ""}
                    </option>
                  ))}
                </select>
              </Field>
              <div className="sm:col-span-2">
                <PaymentMethodField value={form.payment_method || "Cash"} disabled={!form.client_id} onChange={(v) => set({ payment_method: v })} />
              </div>
              <Field label={t("invEditor.issueDate")}>
                <Input
                  type="date"
                  value={form.issue_date}
                  onChange={(e) => set({ issue_date: e.target.value })}
                />
              </Field>
              <Field label={t("invEditor.dueDate")}>
                <Input
                  type="date"
                  value={form.due_date}
                  onChange={(e) => set({ due_date: e.target.value })}
                />
              </Field>
            </div>
          </Card>

          {/* line items */}
          <Card padding="lg">
            <div className="flex items-center justify-between gap-2 mb-4 flex-wrap">
              <CardTitle>{t("invEditor.lineItems")}</CardTitle>
              <div className="flex items-center gap-2">
                <CatalogPicker onPick={addCatalogItem} currency={form.currency} />
                <ReceiptScanButton
                onParsed={(res) => {
                  set({
                    items: res.lineItems?.length
                      ? res.lineItems.map((li) => ({
                          description: li.description || res.vendor || "Item",
                          quantity: Number(li.quantity) || 1,
                          rate: Number(li.rate) || 0,
                        }))
                      : [
                          {
                            description: res.vendor || "Expense",
                            quantity: 1,
                            rate: Number(res.total) || 0,
                          },
                        ],
                    notes: res.notes || form.notes,
                  });
                }}
                />
              </div>
            </div>

            {/* header row */}
            <div className="hidden sm:grid grid-cols-[1fr_80px_110px_110px_32px] gap-3 px-1 pb-2 text-[11px] uppercase tracking-wider text-[var(--ink-muted)] font-semibold">
              <span>{t("common.description")}</span>
              <span className="text-right">{t("common.qty")}</span>
              <span className="text-right">{t("common.rate")}</span>
              <span className="text-right">{t("common.amount")}</span>
              <span></span>
            </div>

            <div className="space-y-2">
              {form.items.map((it, i) => (
                <div
                  key={i}
                  className="grid grid-cols-2 sm:grid-cols-[1fr_80px_110px_110px_32px] gap-3 items-center"
                >
                  <Input
                    className="col-span-2 sm:col-span-1 rounded-xl"
                    placeholder={t("invEditor.descPlaceholder")}
                    value={it.description}
                    onChange={(e) => setItem(i, { description: e.target.value })}
                  />
                  <Input
                    className="rounded-xl text-right tabular"
                    type="number"
                    min="0"
                    step="1"
                    value={it.quantity}
                    onChange={(e) => setItem(i, { quantity: e.target.value })}
                  />
                  <Input
                    className="rounded-xl text-right tabular"
                    type="number"
                    min="0"
                    step="0.01"
                    value={it.rate}
                    onChange={(e) => setItem(i, { rate: e.target.value })}
                  />
                  <div className="text-right text-sm font-semibold tabular text-[var(--ink)] pr-1">
                    {formatMoney(decimal(it.quantity).times(decimal(it.rate)).toString(), form.currency)}
                  </div>
                  <button type="button"
                    onClick={() => removeItem(i)}
                    className="h-8 w-8 rounded-full flex items-center justify-center text-[var(--ink-muted)] hover:text-[var(--danger)] hover:bg-[var(--surface-2)] justify-self-end"
                    title={t("invEditor.removeLine")}
                  >
                    <Trash2 size={14} />
                  </button>
                </div>
              ))}
            </div>

            <Button variant="ghost" size="sm" className="mt-3" onClick={addItem}>
              <Plus size={14} /> {t("invEditor.addLineItem")}
            </Button>
          </Card>

          {/* notes */}
          <Card padding="lg" className="space-y-4">
            <NoteField
              label={t("common.notes")}
              value={form.notes}
              onChange={(v) => set({ notes: v })}
              placeholder={t("invEditor.notesPlaceholder")}
              aiKind="description"
              aiContext={{ items: form.items, client: clientById(clients, form.client_id) }}
            />
            <NoteField
              label={t("common.terms")}
              value={form.terms}
              onChange={(v) => set({ terms: v })}
              placeholder={t("invEditor.termsPlaceholder")}
              aiKind="terms"
              aiContext={{ items: form.items }}
            />
          </Card>
        </div>

        {/* Right — totals */}
        <div className="space-y-5">
          <Card padding="lg" className="lg:sticky lg:top-4">
            <CardTitle className="mb-4">{t("invEditor.summary")}</CardTitle>
            <div className="grid grid-cols-2 gap-3 mb-4">
              <Field label={t("common.currency")}>
                <select
                  className={selectClass}
                  value={form.currency}
                  onChange={(e) => set({ currency: e.target.value })}
                >
                  {CURRENCIES.map((c) => (
                    <option key={c.code} value={c.code}>
                      {c.code}
                    </option>
                  ))}
                </select>
              </Field>
              <Field label={t("invEditor.taxPct")}>
                <Input
                  type="number"
                  min="0"
                  step="0.1"
                  value={form.tax_rate}
                  onChange={(e) => set({ tax_rate: e.target.value })}
                  className="tabular"
                />
              </Field>
            </div>
            <Field label={t("invEditor.discountSymbol", { symbol })} className="mb-4">
              <Input
                type="number"
                min="0"
                step="0.01"
                value={form.discount}
                onChange={(e) => set({ discount: e.target.value })}
                className="tabular"
              />
            </Field>

            <div className="space-y-2 pt-4 border-t border-[var(--border)] text-sm">
              <Row label={t("common.subtotal")} value={formatMoney(totals.subtotal, form.currency)} />
              {totals.discount > 0 && (
                <Row label={t("common.discount")} value={`− ${formatMoney(totals.discount, form.currency)}`} />
              )}
              <Row
                label={t("invEditor.taxLine", { n: Number(form.tax_rate) || 0 })}
                value={formatMoney(totals.taxAmount, form.currency)}
              />
              <div className="flex items-center justify-between pt-3 mt-1 border-t border-[var(--border)]">
                <span className="font-display font-semibold">{t("common.total")}</span>
                <span className="font-display text-xl font-semibold tabular text-[var(--accent-strong)]">
                  {formatMoney(totals.total, form.currency)}
                </span>
              </div>
            </div>
          </Card>
        </div>
      </div>
    </div>
  );
}

function clientById(clients, id) {
  return (clients || []).find((c) => c.id === id) || null;
}

function Field({ label, children, className }) {
  return (
    <label className={cn("block", className)}>
      <span className="block text-xs font-medium text-[var(--ink-muted)] mb-1.5">{label}</span>
      {children}
    </label>
  );
}

function Row({ label, value }) {
  return (
    <div className="flex items-center justify-between">
      <span className="text-[var(--ink-muted)]">{label}</span>
      <span className="tabular text-[var(--ink)]">{value}</span>
    </div>
  );
}

function NoteField({ label, value, onChange, placeholder, aiKind, aiContext }) {
  const { t } = useLang();
  const [loading, setLoading] = useState(false);
  const [unavailable, setUnavailable] = useState(false);
  const [err, setErr] = useState("");
  async function writeWithAI() {
    setLoading(true);
    setUnavailable(false);
    setErr("");
    try {
      const text = await aiApi.writeNote({
        kind: aiKind,
        prompt: value?.trim() || undefined,
        items: (aiContext?.items || []).filter((it) => it.description),
        client: aiContext?.client ? { name: aiContext.client.name } : undefined,
      });
      onChange(text);
    } catch (error) {
      setUnavailable(isAiUnavailable(error));
      if (error.status !== 401) setErr(isAiUnavailable(error) ? t("ai.unavailable") : isAiRateLimited(error) ? t("ai.rateLimited") : isAiFailure(error) ? t("ai.failed") : error.message || t("invEditor.writeFailed"));
    } finally {
      setLoading(false);
    }
  }
  return (
    <div>
      <div className="flex items-center justify-between mb-1.5">
        <span className="text-xs font-medium text-[var(--ink-muted)]">{label}</span>
        <button type="button"
          onClick={writeWithAI}
          disabled={loading || unavailable}
          className="inline-flex items-center gap-1 text-[11px] font-semibold text-[var(--accent-strong)] hover:underline disabled:opacity-50"
        >
          {loading ? <Loader2 size={11} className="animate-spin" /> : <Sparkles size={11} />}
          {unavailable ? t("ai.unavailableShort") : t("invEditor.writeWithAI")}
        </button>
      </div>
      {err && <p className="text-[11px] text-[var(--ink-muted)] mb-1.5">{err}</p>}
      <textarea
        rows={3}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        className="w-full rounded-2xl border border-[var(--border)] bg-[var(--surface)] px-4 py-3 text-sm text-[var(--ink)] placeholder:text-[var(--ink-muted)] outline-none resize-y focus:border-[var(--accent)]/50 focus:ring-2 focus:ring-[var(--accent)]/15"
      />
    </div>
  );
}
