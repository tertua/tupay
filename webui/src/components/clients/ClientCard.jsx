import { ArrowRight } from "lucide-react";
import { Card } from "@/components/ui/Card";
import { ClientStatusBadge } from "@/components/ui/ClientStatusBadge";
import { useLang } from "@/context/LangContext";
import { formatMoney } from "@/lib/utils";

// ClientCard is one client tile in the Clients grid: identity, lifecycle
// badge, and billed/outstanding aggregates.
export function ClientCard({ client, onOpen }) {
  const { t } = useLang();

  return (
    <Card
      padding="lg"
      className="cursor-pointer group focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--accent)]/40"
      onClick={() => onOpen(client.id)}
      role="button"
      tabIndex={0}
      onKeyDown={(e) => {
        if (e.target.closest?.("button")) return;
        if (e.key === "Enter" || e.key === " ") {
          e.preventDefault();
          onOpen(client.id);
        }
      }}
    >
      <div className="flex items-start gap-3">
        <div className="h-11 w-11 rounded-full bg-[var(--accent-soft)] text-[var(--accent-strong)] flex items-center justify-center font-semibold shrink-0">
          {client.name?.[0]?.toUpperCase() || "?"}
        </div>
        <div className="min-w-0 flex-1">
          <div className="font-semibold text-[var(--ink)] truncate group-hover:text-[var(--accent-strong)]">
            {client.name}
          </div>
          <div className="text-xs text-[var(--ink-muted)] truncate">
            {client.company || client.email || "—"}
          </div>
        </div>
        <ArrowRight
          size={16}
          className="text-[var(--ink-muted)] opacity-0 group-hover:opacity-100 transition-opacity shrink-0"
        />
      </div>

      <div className="mt-3">
        <ClientStatusBadge status={client.status} />
      </div>

      <div className="grid grid-cols-2 gap-3 mt-5 pt-4 border-t border-[var(--border)]">
        <div>
          <div className="text-[10px] uppercase tracking-wider text-[var(--ink-muted)] font-semibold">
            {t("common.totalBilled")}
          </div>
          <div className="text-sm font-semibold text-[var(--ink)] tabular mt-0.5">
            {formatMoney(client.total_billed)}
          </div>
        </div>
        <div>
          <div className="text-[10px] uppercase tracking-wider text-[var(--ink-muted)] font-semibold">
            {t("common.outstanding")}
          </div>
          <div
            className={`text-sm font-semibold tabular mt-0.5 ${
              client.outstanding > 0 ? "text-[var(--warning)]" : "text-[var(--ink)]"
            }`}
          >
            {formatMoney(client.outstanding)}
          </div>
        </div>
      </div>
    </Card>
  );
}
