import { useQuery } from "@tanstack/react-query";
import { Loader2, Mail } from "lucide-react";
import { Button } from "@/components/ui/Button";
import { authApi } from "@/api/auth";
import { useInvoiceSendEmail } from "@/hooks/useInvoiceSendEmail";
import { useLang } from "@/context/LangContext";

// Owner-only "send via email" button for one invoice: hidden unless the caller
// owns the org and the invoice is at least sent (a draft must be sent first).
// The backend re-checks both, so this is a convenience guard, not security.
export function InvoiceSendEmailButton({ invoice }) {
  const { t } = useLang();
  const send = useInvoiceSendEmail();
  const { data: role, isLoading } = useQuery({
    queryKey: ["auth", "orgRole"],
    queryFn: () => authApi.me().then((r) => r.org?.role || ""),
    staleTime: 60_000,
  });

  const status = invoice?.status;
  if (!invoice || role !== "owner" || status === "draft" || !invoice.client_id) return null;

  return (
    <Button
      variant="outline"
      disabled={send.isPending || isLoading}
      onClick={() => send.mutate(invoice.id)}
    >
      {send.isPending ? <Loader2 size={14} className="animate-spin" /> : <Mail size={15} />}
      {t("invDetail.sendEmail")}
    </Button>
  );
}
