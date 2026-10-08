import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { subscriptionsApi } from "@/api/subscriptions";
import { useLang } from "@/context/LangContext";

// Cache keys mirror useInvoices: a list family plus a per-id detail key, so a
// mutation can invalidate exactly the affected slices.
export const subscriptionsKey = (params) => ["subscriptions", params || {}];
export const subscriptionKey = (id) => ["subscription", id];

export function useSubscriptions(params) {
  return useQuery({
    queryKey: subscriptionsKey(params),
    queryFn: () => subscriptionsApi.list(params),
  });
}

export function useSubscription(id) {
  return useQuery({
    queryKey: subscriptionKey(id),
    queryFn: () => subscriptionsApi.get(id),
    enabled: !!id,
  });
}

// Every write re-syncs the list family; detail-keyed writes also refresh the
// single subscription. Subscriptions never touch invoice aggregates —
// generation runs in the worker, not on save — so no ["invoices"] invalidation
// here.
function invalidateSubscriptions(qc) {
  qc.invalidateQueries({ queryKey: ["subscriptions"] });
}

export function useCreateSubscription() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (payload) => subscriptionsApi.create(payload),
    onSuccess: () => invalidateSubscriptions(qc),
  });
}

export function useUpdateSubscription() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, payload }) => subscriptionsApi.update(id, payload),
    onSuccess: (sub) => {
      invalidateSubscriptions(qc);
      if (sub?.id) qc.invalidateQueries({ queryKey: subscriptionKey(sub.id) });
    },
  });
}

// Pause/resume is the highest-traffic toggle and its caller (the row action)
// owns no error UI, so the hook toasts on failure — mirroring useSetInvoiceStatus.
export function useSetSubscriptionStatus() {
  const qc = useQueryClient();
  const { t } = useLang();
  return useMutation({
    mutationFn: ({ id, status }) => subscriptionsApi.setStatus(id, status),
    onError: (_err, { id }) => toast.error(t("subscriptions.statusFailed"), { id: `subscription-status-${id}` }),
    onSettled: (_data, _err, { id }) => {
      invalidateSubscriptions(qc);
      qc.invalidateQueries({ queryKey: subscriptionKey(id) });
    },
  });
}

export function useDeleteSubscription() {
  const qc = useQueryClient();
  const { t } = useLang();
  return useMutation({
    mutationFn: (id) => subscriptionsApi.remove(id),
    onError: (_err, id) => toast.error(t("subscriptions.deleteFailed"), { id: `subscription-delete-${id}` }),
    onSettled: () => invalidateSubscriptions(qc),
  });
}
