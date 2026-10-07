import { useQuery } from "@tanstack/react-query";
import { publicClientApi } from "@/api/publicClient";

// Query-key factory: one domain per key so a token change refetches cleanly.
export const publicClientKey = (token) => ["public-client", token];
export const publicClientInvoiceKey = (token, id) => ["public-client", token, "invoice", id];

export function usePublicClient(token) {
  return useQuery({
    queryKey: publicClientKey(token),
    queryFn: () => publicClientApi.get(token),
    enabled: !!token,
  });
}

// Fetched on demand (the PDF button) so the portal's first paint never waits
// on a per-invoice detail call.
export function usePublicClientInvoice(token, id, enabled = true) {
  return useQuery({
    queryKey: publicClientInvoiceKey(token, id),
    queryFn: () => publicClientApi.invoice(token, id),
    enabled: !!token && !!id && enabled,
  });
}
