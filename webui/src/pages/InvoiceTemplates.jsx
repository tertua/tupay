import { useState } from "react";
import { Plus } from "lucide-react";
import { PageHeader } from "@/components/layout/PageHeader";
import { Button } from "@/components/ui/Button";
import { EmptyState } from "@/components/ui/EmptyState";
import { TableSkeleton } from "@/components/ui/TableSkeleton";
import { QueryError } from "@/components/ui/QueryError";
import { InvoicesIllo } from "@/components/ui/EmptyIllustrations";
import { InvoiceTemplateForm } from "@/components/invoice/InvoiceTemplateForm";
import { InvoiceTemplateTable } from "@/components/invoice/InvoiceTemplateTable";
import { useLang } from "@/context/LangContext";
import {
  useInvoiceTemplates,
  useInvoiceTemplate,
  useSetInvoiceTemplateStatus,
  useDeleteInvoiceTemplate,
} from "@/hooks/useInvoiceTemplates";

export default function InvoiceTemplates() {
  const { t } = useLang();
  const { data, isLoading, error } = useInvoiceTemplates();
  const [editing, setEditing] = useState(null); // null | "new" | {id}
  const detail = useInvoiceTemplate(editing && editing !== "new" ? editing.id : null);
  const setStatus = useSetInvoiceTemplateStatus();
  const del = useDeleteInvoiceTemplate();

  const templates = data || [];

  // Only fetch the detail once for edit; hold the form until it resolves.
  const editingTemplate = editing && editing !== "new" ? detail.data : null;
  const formOpen = editing === "new" || (editing && editingTemplate);

  async function onToggle(tpl) {
    await setStatus.mutateAsync({ id: tpl.id, status: tpl.status === "active" ? "paused" : "active" });
  }

  async function onDelete(tpl) {
    if (!window.confirm(t("templates.confirmDelete", { name: tpl.name }))) return;
    await del.mutateAsync(tpl.id);
  }

  return (
    <div>
      <PageHeader
        title={t("templates.title")}
        description={t("templates.desc")}
        actions={
          !formOpen && (
            <Button variant="accent" onClick={() => setEditing("new")}>
              <Plus size={16} /> {t("templates.create")}
            </Button>
          )
        }
      />

      {formOpen ? (
        <div className="max-w-[820px]">
          <InvoiceTemplateForm
            template={editing === "new" ? null : editingTemplate}
            onDone={() => setEditing(null)}
            onCancel={() => setEditing(null)}
          />
        </div>
      ) : isLoading ? (
        <TableSkeleton rows={5} gridClassName="grid-cols-[1.6fr_1.4fr_1fr_1fr_1fr_auto]" columns={["w-32", "w-28", "w-16", "w-20", "w-16", "w-20"]} />
      ) : error ? (
        <QueryError error={error} invalidate={["invoice-templates"]} />
      ) : templates.length === 0 ? (
        <EmptyState
          illustration={<InvoicesIllo />}
          title={t("templates.noneYet")}
          description={t("templates.createFirst")}
          action={
            <Button variant="accent" onClick={() => setEditing("new")}>
              <Plus size={16} /> {t("templates.create")}
            </Button>
          }
        />
      ) : (
        <InvoiceTemplateTable
          templates={templates}
          onEdit={(tpl) => setEditing({ id: tpl.id })}
          onToggle={onToggle}
          onDelete={onDelete}
        />
      )}
    </div>
  );
}
