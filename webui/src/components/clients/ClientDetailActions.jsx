import { Archive, ArchiveRestore, Plus, Pencil, Trash2, Send } from "lucide-react";
import { Button } from "@/components/ui/Button";
import { useLang } from "@/context/LangContext";
import { useArchiveClient, useUnarchiveClient } from "@/hooks/useClients";
import { useSendClientReminder } from "@/hooks/useClientReceivables";

// ClientDetailActions owns the header action cluster: new invoice, edit,
// archive/unarchive, send reminder and delete. Extracted from ClientDetail.jsx
// to keep that page within its size ratchet.
export function ClientDetailActions({ client, id, onEdit, onDelete, onNewInvoice }) {
  const { t } = useLang();
  const archive = useArchiveClient();
  const unarchive = useUnarchiveClient();
  const reminder = useSendClientReminder();

  const isArchived = client.status === "archived";

  function onToggleArchive() {
    if (isArchived) {
      unarchive.mutate(id);
      return;
    }
    if (window.confirm(t("clientDetail.archiveConfirm", { name: client.name }))) {
      archive.mutate(id);
    }
  }

  return (
    <div className="flex items-center gap-2 flex-wrap">
      <Button variant="accent" onClick={() => onNewInvoice()}>
        <Plus size={15} /> {t("clientDetail.newInvoice")}
      </Button>
      <Button variant="outline" onClick={() => reminder.mutate(id)} disabled={reminder.isPending}>
        <Send size={15} /> {t("clientDetail.sendReminder")}
      </Button>
      <Button variant="outline" onClick={() => onEdit()}>
        <Pencil size={15} /> {t("common.edit")}
      </Button>
      <Button variant="outline" onClick={onToggleArchive} disabled={archive.isPending || unarchive.isPending}>
        {isArchived ? <ArchiveRestore size={15} /> : <Archive size={15} />}
        {isArchived ? t("clientDetail.unarchive") : t("clientDetail.archive")}
      </Button>
      <Button
        variant="ghost"
        onClick={() => onDelete()}
        aria-label={t("common.delete")}
        title={t("common.delete")}
        className="text-[var(--danger)] hover:bg-[var(--danger)]/10"
      >
        <Trash2 size={15} />
      </Button>
    </div>
  );
}
