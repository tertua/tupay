import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Check, Loader2, Send, Undo2, Upload } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/Button";
import { authApi } from "@/api/auth";
import { invoiceApprovalApi } from "@/api/invoiceApproval";
import { invoicesApi } from "@/api/invoices";
import { useLang } from "@/context/LangContext";

// Pending-approval action bar for one invoice: staff submits drafts, the owner approves/rejects pending ones or sends a draft (D11).
export function InvoiceStatusActions({ invoice }) {
  const { t } = useLang();
  const qc = useQueryClient();
  const { data: role, isLoading } = useQuery({
    queryKey: ["auth", "orgRole"],
    queryFn: () => authApi.me().then((r) => r.org?.role || ""),
    staleTime: 60_000,
  });
  const isOwner = role === "owner";
  // Stored status only: a sent invoice with a live gateway transaction also
  // reads effective "pending" but must never offer approve/reject.
  const status = invoice?.status;

  const act = useMutation({
    mutationFn: (job) => job.run(),
    onSuccess: (inv, job) => {
      toast.success(t(job.ok));
      qc.invalidateQueries({ queryKey: ["invoices"] });
      qc.invalidateQueries({ queryKey: ["invoice", inv?.id || invoice.id] });
      qc.invalidateQueries({ queryKey: ["dashboard"] });
    },
    onError: (err, job) => {
      if (err?.status !== 401) toast.error(t(job.fail), { description: err?.message });
    },
  });

  if (!invoice || (status !== "draft" && status !== "pending")) return null;

  const name = invoice.client_name || t("common.noClient");
  const actions = [];
  if (status === "draft" && !isOwner && !isLoading) {
    actions.push({
      key: "submit", label: t("approval.submit"), icon: <Upload size={14} />, variant: "accent",
      confirm: t("approval.confirmSubmit"),
      job: { run: () => invoiceApprovalApi.submit(invoice.id), ok: "approval.submitted", fail: "approval.submitFailed" },
    });
  }
  if (status === "draft" && isOwner) {
    actions.push({
      key: "send", label: t("approval.send"), icon: <Send size={14} />, variant: "accent",
      confirm: t("approval.confirmSend", { name }),
      job: { run: () => invoicesApi.setStatus(invoice.id, "sent"), ok: "approval.sent", fail: "approval.sendFailed" },
    });
  }
  if (status === "pending" && isOwner) {
    actions.push(
      {
        key: "approve", label: t("approval.approve"), icon: <Check size={14} />, variant: "accent",
        confirm: t("approval.confirmApprove"),
        job: { run: () => invoiceApprovalApi.approve(invoice.id), ok: "approval.approved", fail: "approval.approveFailed" },
      },
      {
        key: "reject", label: t("approval.reject"), icon: <Undo2 size={14} />, variant: "outline",
        confirm: t("approval.confirmReject"),
        job: { run: () => invoiceApprovalApi.reject(invoice.id), ok: "approval.rejected", fail: "approval.rejectFailed" },
      }
    );
  }

  const hint =
    status === "pending"
      ? isOwner
        ? t("approval.tooltipOwner")
        : t("approval.waitingOwner")
      : isOwner
        ? t("approval.sendNote")
        : t("approval.tooltipStaff");

  return (
    <div className="flex flex-wrap items-center gap-3 px-4 py-3 mb-4 rounded-2xl border border-[var(--border)] bg-[var(--surface)] shadow-card">
      <span className="text-xs text-[var(--ink-muted)]">{hint}</span>
      <div className="flex items-center gap-2 md:ml-auto">
        {isLoading && <Loader2 size={14} className="animate-spin text-[var(--ink-muted)]" />}
        {actions.map((a) => (
          <Button
            key={a.key}
            size="sm"
            variant={a.variant}
            disabled={act.isPending}
            onClick={() => {
              if (window.confirm(a.confirm)) act.mutate(a.job);
            }}
          >
            {act.isPending ? <Loader2 size={14} className="animate-spin" /> : a.icon}
            {a.label}
          </Button>
        ))}
      </div>
    </div>
  );
}
