import { useEffect, useState } from "react";
import Decimal from "decimal.js";
import { Plus, Trash2, Loader2 } from "lucide-react";
import { Card, CardTitle } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { useClients } from "@/hooks/useClients";
import { useLang } from "@/context/LangContext";
import {
  useCreateSubscription,
  useUpdateSubscription,
} from "@/hooks/useSubscriptions";
import { todayDateInput, toDateInput, cn } from "@/lib/utils";

const decimal = (value) => new Decimal(value || 0);
const blankItem = () => ({ description: "", quantity: 1, rate: 0 });
const CADENCES = ["weekly", "monthly"];
const INVOICE_STATUSES = ["draft", "sent"];

// Create/edit form for a subscription. Kept presentational-plus-one-mutation so
// Subscriptions.jsx stays a thin list page (file-size split, same shape as
// components/clients/ClientFormModal.jsx).
export function SubscriptionForm({ subscription, onDone, onCancel }) {
  const isEdit = !!subscription;
  const { t } = useLang();
  const { data: clientsData } = useClients();
  const clients = clientsData?.clients || [];
  const create = useCreateSubscription();
  const update = useUpdateSubscription();
  const [form, setForm] = useState(null);
  const [saving, setSaving] = useState(false);
  const [err, setErr] = useState("");

  useEffect(() => {
    setForm(
      subscription
        ? {
            name: subscription.name || "",
            client_id: subscription.client_id || "",
            cadence: subscription.cadence || "monthly",
            invoice_status: subscription.invoice_status || "draft",
            start_date: toDateInput(subscription.next_run_date) || "",
            lead_days: subscription.lead_days ?? 0,
            due_days: subscription.due_days ?? 30,
            currency: subscription.currency || "IDR",
            tax_rate: Number(subscription.tax_rate) || 0,
            items: subscription.items?.length
              ? subscription.items.map((it) => ({
                  description: it.description,
                  quantity: it.quantity,
                  rate: it.rate,
                }))
              : [blankItem()],
          }
        : {
            name: "",
            client_id: "",
            cadence: "monthly",
            invoice_status: "draft",
            start_date: todayDateInput(),
            lead_days: 0,
            due_days: 30,
            currency: "IDR",
            tax_rate: 0,
            items: [blankItem()],
          }
    );
    setErr("");
  }, [subscription]);

  if (!form) return null;

  const set = (patch) => setForm((f) => ({ ...f, ...patch }));
  const setItem = (i, patch) =>
    set({ items: form.items.map((it, idx) => (idx === i ? { ...it, ...patch } : it)) });
  const addItem = () => set({ items: [...form.items, blankItem()] });
  const removeItem = (i) =>
    set({ items: form.items.filter((_, idx) => idx !== i).length ? form.items.filter((_, idx) => idx !== i) : [blankItem()] });

  async function onSubmit(e) {
    e.preventDefault();
    if (!form.name.trim()) return setErr(t("common.name") + " — " + t("items.nameRequired"));
    if (form.invoice_status === "sent" && !form.client_id) return setErr(t("subscriptions.clientRequired"));
    setSaving(true);
    setErr("");
    const payload = {
      ...form,
      client_id: form.client_id || null,
      lead_days: Number(form.lead_days) || 0,
      due_days: Number(form.due_days) || 0,
      tax_rate: Number(form.tax_rate) || 0,
      items: form.items
        .filter((it) => it.description.trim() || decimal(it.rate).greaterThan(0))
        .map((it) => ({
          description: it.description,
          quantity: Number(it.quantity) || 0,
          rate: decimal(it.rate).toFixed(4),
        })),
    };
    try {
      if (isEdit) await update.mutateAsync({ id: subscription.id, payload });
      else await create.mutateAsync(payload);
      onDone();
    } catch (ex) {
      if (ex.status !== 401) setErr(ex.message || t("subscriptions.saveFailed"));
    } finally {
      setSaving(false);
    }
  }

  const selectClass =
    "h-10 w-full rounded-full border border-[var(--border)] bg-[var(--surface)] px-4 text-sm text-[var(--ink)] outline-none focus:border-[var(--accent)]/50 focus:ring-2 focus:ring-[var(--accent)]/15";

  return (
    <form onSubmit={onSubmit}>
      <Card padding="lg">
        <CardTitle className="mb-4">{isEdit ? t("subscriptions.editTitle") : t("subscriptions.newTitle")}</CardTitle>
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <Field label={t("common.name")} className="sm:col-span-2">
            <Input value={form.name} onChange={(e) => set({ name: e.target.value })} placeholder={t("subscriptions.namePlaceholder")} />
          </Field>
          <Field label={t("common.client")}>
            <select className={selectClass} value={form.client_id} onChange={(e) => set({ client_id: e.target.value })}>
              <option value="">{t("invEditor.noClient")}</option>
              {(clients || []).map((c) => (
                <option key={c.id} value={c.id}>
                  {c.name}
                  {c.company ? ` (${c.company})` : ""}
                </option>
              ))}
            </select>
          </Field>
          <Field label={t("subscriptions.cadence")}>
            <select className={selectClass} value={form.cadence} onChange={(e) => set({ cadence: e.target.value })}>
              {CADENCES.map((c) => (
                <option key={c} value={c}>{t(`subscriptions.cadence.${c}`)}</option>
              ))}
            </select>
          </Field>
          <Field label={t("subscriptions.startDate")}>
            <Input type="date" value={form.start_date} onChange={(e) => set({ start_date: e.target.value })} />
          </Field>
          <Field label={t("subscriptions.invoiceStatus")}>
            <select className={selectClass} value={form.invoice_status} onChange={(e) => set({ invoice_status: e.target.value })}>
              {INVOICE_STATUSES.map((s) => (
                <option key={s} value={s}>{t(`status.${s}`)}</option>
              ))}
            </select>
          </Field>
          <Field label={t("subscriptions.leadDays")}>
            <Input type="number" min="0" max="90" value={form.lead_days} onChange={(e) => set({ lead_days: e.target.value })} className="tabular" />
          </Field>
          <Field label={t("subscriptions.dueDays")}>
            <Input type="number" min="0" max="365" value={form.due_days} onChange={(e) => set({ due_days: e.target.value })} className="tabular" />
          </Field>
        </div>

        <div className="mt-6 mb-2 flex items-center justify-between">
          <CardTitle>{t("invEditor.lineItems")}</CardTitle>
          <Button type="button" variant="ghost" size="sm" onClick={addItem}>
            <Plus size={14} /> {t("invEditor.addLineItem")}
          </Button>
        </div>
        <div className="space-y-2">
          {form.items.map((it, i) => (
            <div key={i} className="grid grid-cols-2 sm:grid-cols-[1fr_80px_110px_32px] gap-3 items-center">
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

        {err && <p className="text-sm text-[var(--danger)] mt-4">{err}</p>}

        <div className="flex items-center justify-end gap-2 mt-6">
          <Button type="button" variant="outline" onClick={onCancel}>
            {t("common.cancel")}
          </Button>
          <Button type="submit" variant="accent" disabled={saving}>
            {saving && <Loader2 size={14} className="animate-spin" />}
            {isEdit ? t("common.saveChanges") : t("common.add")}
          </Button>
        </div>
      </Card>
    </form>
  );
}

function Field({ label, children, className }) {
  return (
    <label className={cn("block", className)}>
      <span className="block text-xs font-medium text-[var(--ink-muted)] mb-1.5">{label}</span>
      {children}
    </label>
  );
}
