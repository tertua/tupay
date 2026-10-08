import { Pause, Play, Pencil, Trash2 } from "lucide-react";
import { Card } from "@/components/ui/Card";
import { Badge } from "@/components/ui/Badge";
import { useClients } from "@/hooks/useClients";
import { useLang } from "@/context/LangContext";
import { formatDate } from "@/lib/utils";

// Recurring-template list. Status uses a local badge mapping (active/paused)
// rather than StatusBadge, which is keyed to invoice statuses.
const TEMPLATE_STATUS = {
  active: { tone: "success", labelKey: "templates.status.active" },
  paused: { tone: "neutral", labelKey: "templates.status.paused" },
};

export function InvoiceTemplateTable({ templates, onEdit, onToggle, onDelete }) {
  const { t } = useLang();
  const { data: clients } = useClients();
  const clientName = (id) => (clients || []).find((c) => c.id === id)?.name;

  return (
    <Card padding="none" className="overflow-hidden">
      <div className="hidden md:grid grid-cols-[1.6fr_1.4fr_1fr_1fr_1fr_auto] gap-4 px-5 py-3 border-b border-[var(--border)] text-[11px] uppercase tracking-wider text-[var(--ink-muted)] font-semibold">
        <span>{t("common.name")}</span>
        <span>{t("common.client")}</span>
        <span>{t("templates.cadence")}</span>
        <span>{t("templates.nextRun")}</span>
        <span>{t("invoices.colStatus")}</span>
        <span className="text-right">{t("templates.actions")}</span>
      </div>

      <div className="divide-y divide-[var(--border)]">
        {templates.map((tpl) => {
          const active = tpl.status === "active";
          const s = TEMPLATE_STATUS[tpl.status] || TEMPLATE_STATUS.paused;
          return (
            <div
              key={tpl.id}
              className="grid grid-cols-2 md:grid-cols-[1.6fr_1.4fr_1fr_1fr_1fr_auto] gap-x-4 gap-y-1 px-5 py-4 items-center"
            >
              <div className="font-semibold text-sm text-[var(--ink)] truncate">{tpl.name}</div>
              <div className="text-sm text-[var(--ink)] truncate order-3 md:order-none col-span-2 md:col-span-1">
                {clientName(tpl.client_id) || <span className="text-[var(--ink-muted)]">{t("common.noClient")}</span>}
              </div>
              <div className="hidden md:block text-sm text-[var(--ink-muted)]">
                {t(`templates.cadence.${tpl.cadence}`)}
              </div>
              <div className="hidden md:block text-sm text-[var(--ink-muted)] tabular">
                {tpl.next_run_date ? formatDate(tpl.next_run_date) : "—"}
              </div>
              <div>
                <Badge tone={s.tone}>{t(s.labelKey)}</Badge>
              </div>
              <div className="flex items-center justify-end gap-0.5">
                <button type="button"
                  onClick={() => onToggle(tpl)}
                  title={active ? t("templates.pause") : t("templates.resume")}
                  className="h-7 w-7 rounded-full flex items-center justify-center text-[var(--ink-muted)] hover:bg-[var(--surface-2)] hover:text-[var(--ink)]"
                >
                  {active ? <Pause size={13} /> : <Play size={13} />}
                </button>
                <button type="button"
                  onClick={() => onEdit(tpl)}
                  title={t("common.edit")}
                  className="h-7 w-7 rounded-full flex items-center justify-center text-[var(--ink-muted)] hover:bg-[var(--surface-2)] hover:text-[var(--ink)]"
                >
                  <Pencil size={13} />
                </button>
                <button type="button"
                  onClick={() => onDelete(tpl)}
                  title={t("common.delete")}
                  className="h-7 w-7 rounded-full flex items-center justify-center text-[var(--ink-muted)] hover:bg-[var(--surface-2)] hover:text-[var(--danger)]"
                >
                  <Trash2 size={13} />
                </button>
              </div>
            </div>
          );
        })}
      </div>
    </Card>
  );
}
