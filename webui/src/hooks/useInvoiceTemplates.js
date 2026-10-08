import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { invoiceTemplatesApi } from "@/api/invoiceTemplates";
import { useLang } from "@/context/LangContext";

// Cache keys mirror useInvoices: a list family plus a per-id detail key, so a
// mutation can invalidate exactly the affected slices.
export const invoiceTemplatesKey = (params) => ["invoice-templates", params || {}];
export const invoiceTemplateKey = (id) => ["invoice-template", id];

export function useInvoiceTemplates(params) {
  return useQuery({
    queryKey: invoiceTemplatesKey(params),
    queryFn: () => invoiceTemplatesApi.list(params),
  });
}

export function useInvoiceTemplate(id) {
  return useQuery({
    queryKey: invoiceTemplateKey(id),
    queryFn: () => invoiceTemplatesApi.get(id),
    enabled: !!id,
  });
}

// Every write re-syncs the list family; detail-keyed writes also refresh the
// single template. Templates never touch invoice aggregates — generation runs
// in the worker, not on save — so no ["invoices"] invalidation here.
function invalidateTemplates(qc) {
  qc.invalidateQueries({ queryKey: ["invoice-templates"] });
}

export function useCreateInvoiceTemplate() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (payload) => invoiceTemplatesApi.create(payload),
    onSuccess: () => invalidateTemplates(qc),
  });
}

export function useUpdateInvoiceTemplate() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, payload }) => invoiceTemplatesApi.update(id, payload),
    onSuccess: (tpl) => {
      invalidateTemplates(qc);
      if (tpl?.id) qc.invalidateQueries({ queryKey: invoiceTemplateKey(tpl.id) });
    },
  });
}

// Pause/resume is the highest-traffic toggle and its caller (the row action)
// owns no error UI, so the hook toasts on failure — mirroring useSetInvoiceStatus.
export function useSetInvoiceTemplateStatus() {
  const qc = useQueryClient();
  const { t } = useLang();
  return useMutation({
    mutationFn: ({ id, status }) => invoiceTemplatesApi.setStatus(id, status),
    onError: (_err, { id }) => toast.error(t("templates.statusFailed"), { id: `template-status-${id}` }),
    onSettled: (_data, _err, { id }) => {
      invalidateTemplates(qc);
      qc.invalidateQueries({ queryKey: invoiceTemplateKey(id) });
    },
  });
}

export function useDeleteInvoiceTemplate() {
  const qc = useQueryClient();
  const { t } = useLang();
  return useMutation({
    mutationFn: (id) => invoiceTemplatesApi.remove(id),
    onError: (_err, id) => toast.error(t("templates.deleteFailed"), { id: `template-delete-${id}` }),
    onSettled: () => invalidateTemplates(qc),
  });
}
