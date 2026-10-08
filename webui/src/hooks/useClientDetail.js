import { useQuery } from "@tanstack/react-query";
import { clientsApi } from "@/api/clients";
import { clientKey } from "./useClients";

// useClient loads one client's detail. When params are passed (e.g. the
// overdue-only toggle) they extend the cache key so each view is cached
// separately; without params the base ["client", id] key is kept so the
// optimistic helpers and invalidation keep matching.
export function useClient(id, params) {
  return useQuery({
    queryKey: params ? [...clientKey(id), params] : clientKey(id),
    queryFn: () => clientsApi.get(id, params),
    enabled: !!id,
  });
}
