import { useMutation, useQueryClient } from "@tanstack/react-query";
import { clientPortalApi } from "@/api/clientPortal";
import { clientKey } from "./useClients";

// All three portal mutations refresh the client detail so the card reflects
// the new link state without a manual reload.
function invalidate(qc, id) {
  if (id) qc.invalidateQueries({ queryKey: clientKey(id) });
}

export function useEnsureClientPortal() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id) => clientPortalApi.ensure(id),
    onSettled: (_data, _err, id) => invalidate(qc, id),
  });
}

export function useRegenerateClientPortal() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id) => clientPortalApi.regenerate(id),
    onSettled: (_data, _err, id) => invalidate(qc, id),
  });
}

export function useRevokeClientPortal() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id) => clientPortalApi.revoke(id),
    onSettled: (_data, _err, id) => invalidate(qc, id),
  });
}
