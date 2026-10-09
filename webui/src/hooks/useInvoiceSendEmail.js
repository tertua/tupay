import { useMutation } from "@tanstack/react-query";
import { toast } from "sonner";
import { invoiceSendEmailApi } from "@/api/invoiceSendEmail";
import { useLang } from "@/context/LangContext";

// Emails an invoice to the client; the toast is the only UI feedback and a
// second click is a legitimate resend (no optimistic state).
export function useInvoiceSendEmail() {
  const { t } = useLang();
  return useMutation({
    mutationFn: (id) => invoiceSendEmailApi.send(id),
    onSuccess: () => toast.success(t("invDetail.emailQueued")),
    onError: (err) => {
      if (err?.status !== 401) {
        const key = err?.status === 409 ? "invDetail.emailConflict"
          : err?.status === 501 ? "invDetail.emailUnconfigured"
          : "invDetail.emailFailed";
        toast.error(t(key), { description: err?.message });
      }
    },
  });
}
