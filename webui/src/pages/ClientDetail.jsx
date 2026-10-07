import { lazy, Suspense, useMemo, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import {
  ArrowLeft,
  Pencil,
  Trash2,
  Plus,
  Loader2,
  Mail,
  Phone,
  MapPin,
  Building2,
} from "lucide-react";
import { Card, CardTitle } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { StatusBadge } from "@/components/ui/Badge";
import { EmptyState } from "@/components/ui/EmptyState";
import { ClientFormModal } from "@/components/clients/ClientFormModal";
import ClientPortalCard from "@/components/clients/ClientPortalCard";
import { ChartFallback, ContactRow, MiniStat } from "@/components/clients/ClientDetailParts";
import { useClient, useDeleteClient } from "@/hooks/useClients";
import { useLang } from "@/context/LangContext";
import { formatMoney, formatDate, formatMonthShort, todayDateInput } from "@/lib/utils";

const ClientCharts = lazy(() => import("@/components/clients/ClientCharts"));

function displayStatus(inv) {
  return inv.effective_status || inv.status;
}

function isOverdue(inv) {
  return displayStatus(inv) === "sent" && inv.due_date && inv.due_date < todayDateInput();
}

// Everything below is derived from the invoices already loaded — no extra API call.
function computeInsights(invoices, stats, t) {
  const num = (v) => Number(v) || 0;
  const paidAmt = invoices.filter((i) => displayStatus(i) === "paid").reduce((s, i) => s + num(i.total), 0);
  const overdueAmt = invoices.filter(isOverdue).reduce((s, i) => s + num(i.total), 0);
  const openAmt = Math.max(0, num(stats.totalBilled) - paidAmt - overdueAmt);

  const breakdown = [
    { name: t("clientDetail.statusPaid"), value: Math.round(paidAmt * 100) / 100, color: "var(--success)" },
    { name: t("clientDetail.statusOpen"), value: Math.round(openAmt * 100) / 100, color: "var(--accent)" },
    { name: t("clientDetail.statusOverdue"), value: Math.round(overdueAmt * 100) / 100, color: "var(--danger)" },
  ].filter((s) => s.value > 0);

  // Last 6 months of billing by issue date.
  const map = {};
  invoices.forEach((inv) => {
    const d = new Date(inv.issue_date);
    if (Number.isNaN(d.getTime())) return;
    const key = `${d.getFullYear()}-${d.getMonth()}`;
    map[key] = (map[key] || 0) + num(inv.total);
  });
  const now = new Date();
  const monthly = [];
  for (let i = 5; i >= 0; i--) {
    const d = new Date(now.getFullYear(), now.getMonth() - i, 1);
    const key = `${d.getFullYear()}-${d.getMonth()}`;
    monthly.push({
      label: formatMonthShort(d),
      value: Math.round((map[key] || 0) * 100) / 100,
    });
  }

  const paidCount = invoices.filter((i) => displayStatus(i) === "paid").length;
  const count = invoices.length;
  const largest = invoices.reduce((m, i) => Math.max(m, num(i.total)), 0);
  const avgInvoice = count ? num(stats.totalBilled) / count : 0;
  const paidRate = count ? Math.round((paidCount / count) * 100) : 0;

  return { breakdown, monthly, largest, avgInvoice, paidRate, hasMonthly: monthly.some((m) => m.value > 0) };
}

export default function ClientDetail() {
  const { id } = useParams();
  const nav = useNavigate();
  const { t } = useLang();
  const { data, isLoading, error } = useClient(id);
  const del = useDeleteClient();
  const [editOpen, setEditOpen] = useState(false);

  const invoiceList = useMemo(() => data?.invoices || [], [data?.invoices]);
  const clientStats = useMemo(
    () => data?.stats || { count: 0, totalBilled: 0, outstanding: 0 },
    [data?.stats]
  );
  const insights = useMemo(() => computeInsights(invoiceList, clientStats, t), [invoiceList, clientStats, t]);

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-24 text-[var(--ink-muted)]">
        <Loader2 className="animate-spin" size={20} />
      </div>
    );
  }
  if (error?.status === 401) return null;
  if (error || !data?.client) {
    return <EmptyState icon={Mail} title={t("clientDetail.notFound")} description={t("clientDetail.deleted")} />;
  }

  const client = data.client;

  async function onDelete() {
    if (!window.confirm(t("clientDetail.confirmDelete", { name: client.name }))) return;
    await del.mutateAsync(id);
    nav("/clients");
  }

  return (
    <div>
      <div className="flex items-center justify-between gap-4 mb-6 flex-wrap">
        <div className="flex items-center gap-3">
          <button type="button"
            onClick={() => nav("/clients")}
            className="h-9 w-9 rounded-full flex items-center justify-center border border-[var(--border)] bg-[var(--surface)] text-[var(--ink-muted)] hover:text-[var(--ink)] shadow-card"
          >
            <ArrowLeft size={16} />
          </button>
          <div className="flex items-center gap-3">
            <div className="h-12 w-12 rounded-full bg-[var(--accent-soft)] text-[var(--accent-strong)] flex items-center justify-center font-semibold text-lg">
              {client.name?.[0]?.toUpperCase() || "?"}
            </div>
            <div>
              <h2 className="font-display text-2xl font-semibold tracking-tight">{client.name}</h2>
              {client.company && <p className="text-sm text-[var(--ink-muted)]">{client.company}</p>}
            </div>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <Button
            variant="accent"
            onClick={() => nav(`/invoices/new?client=${id}`)}
          >
            <Plus size={15} /> {t("clientDetail.newInvoice")}
          </Button>
          <Button variant="outline" onClick={() => setEditOpen(true)}>
            <Pencil size={15} /> {t("common.edit")}
          </Button>
          <Button variant="ghost" onClick={onDelete} aria-label={t("common.delete")} title={t("common.delete")} className="text-[var(--danger)] hover:bg-[var(--danger)]/10">
            <Trash2 size={15} />
          </Button>
        </div>
      </div>

      {/* Stats — full-width row so currency values have room */}
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 mb-5">
        <MiniStat label={t("clientDetail.invoices")} value={clientStats.count} />
        <MiniStat label={t("common.totalBilled")} value={formatMoney(clientStats.totalBilled)} />
        <MiniStat label={t("common.outstanding")} value={formatMoney(clientStats.outstanding)} warn={clientStats.outstanding > 0} />
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-5 items-start">
        {/* Left: contact */}
        <div className="space-y-5">
          <Card padding="lg">
            <CardTitle className="mb-4">{t("clientDetail.contact")}</CardTitle>
            <div className="space-y-3">
              <ContactRow icon={Mail} value={client.email} href={client.email ? `mailto:${client.email}` : null} />
              <ContactRow icon={Phone} value={client.phone} />
              <ContactRow icon={Building2} value={client.company} />
              <ContactRow icon={MapPin} value={client.address} />
            </div>
            {client.notes && (
              <div className="mt-4 pt-4 border-t border-[var(--border)]">
                <div className="text-[10px] uppercase tracking-wider text-[var(--ink-muted)] font-semibold mb-1">
                  {t("common.notes")}
                </div>
                <p className="text-sm text-[var(--ink)]">{client.notes}</p>
              </div>
            )}
          </Card>
          <ClientPortalCard clientId={id} />
        </div>

        {/* Right: invoice history */}
        <div className="lg:col-span-2">
          <Card padding="lg">
            <CardTitle className="mb-4">{t("clientDetail.invoiceHistory")}</CardTitle>
            {invoiceList.length === 0 ? (
              <div className="py-10 text-center">
                <p className="text-sm text-[var(--ink-muted)]">{t("clientDetail.noInvoices")}</p>
                <Button variant="soft" size="sm" className="mt-3" onClick={() => nav(`/invoices/new?client=${id}`)}>
                  <Plus size={14} /> {t("clientDetail.createOne")}
                </Button>
              </div>
            ) : (
              <div className="divide-y divide-[var(--border)]">
                {invoiceList.map((inv) => (
                  <button type="button"
                    key={inv.id}
                    onClick={() => nav(`/invoices/${inv.id}`)}
                    className="w-full flex items-center gap-3 py-3 text-left hover:opacity-90"
                  >
                    <div className="flex-1 min-w-0">
                      <div className="text-sm font-semibold text-[var(--ink)] tabular">
                        {inv.invoice_number}
                      </div>
                      <div className="text-xs text-[var(--ink-muted)]">
                        {t("clientDetail.issuedDue", { issued: formatDate(inv.issue_date), due: formatDate(inv.due_date) })}
                      </div>
                    </div>
                    <div className="text-sm font-semibold text-[var(--ink)] tabular shrink-0">
                      {formatMoney(inv.total, inv.currency)}
                    </div>
                    <StatusBadge
                      status={isOverdue(inv) ? "overdue" : displayStatus(inv)}
                    />
                  </button>
                ))}
              </div>
            )}
          </Card>
        </div>
      </div>

      {/* Insights row — aligns with the columns above (1 / 2 split) */}
      {invoiceList.length > 0 && (
        <Suspense fallback={<ChartFallback />}>
          <ClientCharts insights={insights} />
        </Suspense>
      )}

      <ClientFormModal open={editOpen} onClose={() => setEditOpen(false)} client={client} />
    </div>
  );
}
