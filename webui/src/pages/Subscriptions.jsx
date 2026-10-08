import { useState } from "react";
import { Plus } from "lucide-react";
import { PageHeader } from "@/components/layout/PageHeader";
import { Button } from "@/components/ui/Button";
import { EmptyState } from "@/components/ui/EmptyState";
import { TableSkeleton } from "@/components/ui/TableSkeleton";
import { QueryError } from "@/components/ui/QueryError";
import { InvoicesIllo } from "@/components/ui/EmptyIllustrations";
import { SubscriptionForm } from "@/components/invoice/SubscriptionForm";
import { SubscriptionTable } from "@/components/invoice/SubscriptionTable";
import { useLang } from "@/context/LangContext";
import {
  useSubscriptions,
  useSubscription,
  useSetSubscriptionStatus,
  useDeleteSubscription,
} from "@/hooks/useSubscriptions";

export default function Subscriptions() {
  const { t } = useLang();
  const { data, isLoading, error } = useSubscriptions();
  const [editing, setEditing] = useState(null); // null | "new" | {id}
  const detail = useSubscription(editing && editing !== "new" ? editing.id : null);
  const setStatus = useSetSubscriptionStatus();
  const del = useDeleteSubscription();

  const subscriptions = data || [];

  // Only fetch the detail once for edit; hold the form until it resolves.
  const editingSubscription = editing && editing !== "new" ? detail.data : null;
  const formOpen = editing === "new" || (editing && editingSubscription);

  async function onToggle(sub) {
    await setStatus.mutateAsync({ id: sub.id, status: sub.status === "active" ? "paused" : "active" });
  }

  async function onDelete(sub) {
    if (!window.confirm(t("subscriptions.confirmDelete", { name: sub.name }))) return;
    await del.mutateAsync(sub.id);
  }

  return (
    <div>
      <PageHeader
        title={t("subscriptions.title")}
        description={t("subscriptions.desc")}
        actions={
          !formOpen && (
            <Button variant="accent" onClick={() => setEditing("new")}>
              <Plus size={16} /> {t("subscriptions.create")}
            </Button>
          )
        }
      />

      {formOpen ? (
        <div className="max-w-[820px]">
          <SubscriptionForm
            subscription={editing === "new" ? null : editingSubscription}
            onDone={() => setEditing(null)}
            onCancel={() => setEditing(null)}
          />
        </div>
      ) : isLoading ? (
        <TableSkeleton rows={5} gridClassName="grid-cols-[1.6fr_1.4fr_1fr_1fr_1fr_auto]" columns={["w-32", "w-28", "w-16", "w-20", "w-16", "w-20"]} />
      ) : error ? (
        <QueryError error={error} invalidate={["subscriptions"]} />
      ) : subscriptions.length === 0 ? (
        <EmptyState
          illustration={<InvoicesIllo />}
          title={t("subscriptions.noneYet")}
          description={t("subscriptions.createFirst")}
          action={
            <Button variant="accent" onClick={() => setEditing("new")}>
              <Plus size={16} /> {t("subscriptions.create")}
            </Button>
          }
        />
      ) : (
        <SubscriptionTable
          subscriptions={subscriptions}
          onEdit={(sub) => setEditing({ id: sub.id })}
          onToggle={onToggle}
          onDelete={onDelete}
        />
      )}
    </div>
  );
}
