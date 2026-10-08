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
