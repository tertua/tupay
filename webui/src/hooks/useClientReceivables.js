import { useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { clientsApi } from "@/api/clients";
import { useLang } from "@/context/LangContext";

// useSendClientReminder queues manual payment reminders for a client's open
// invoices; the toast reports how many were queued.
export function useSendClientReminder() {
  const qc = useQueryClient();
  const { t } = useLang();
  return useMutation({
    mutationFn: (id) => clientsApi.sendReminder(id),
    onSuccess: (data) => {
      toast.success(t("clientDetail.reminderQueued", { count: data?.queued ?? 0 }));
      qc.invalidateQueries({ queryKey: ["client"] });
    },
    onError: () => toast.error(t("clientDetail.reminderFailed")),
  });
}

// useExportClientStatement downloads a client's invoice statement as a CSV
// blob and saves it; the filename is derived client-side (the .csv endpoint
// bypasses the JSON envelope).
export function useExportClientStatement() {
  const { t } = useLang();
  return useMutation({
    mutationFn: ({ id, name }) => clientsApi.statement(id).then((blob) => ({ blob, name })),
    onSuccess: ({ blob, name }) => {
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `client-${slugify(name)}-statement.csv`;
      document.body.appendChild(a);
      a.click();
      a.remove();
      setTimeout(() => URL.revokeObjectURL(url), 4000);
    },
    onError: () => toast.error(t("clientDetail.exportFailed")),
  });
}

// slugify reduces a client name to a filename-safe token.
function slugify(name) {
  const out = String(name || "")
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .slice(0, 48);
  return out || "client";
}
