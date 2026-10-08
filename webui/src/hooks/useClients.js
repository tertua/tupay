import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { clientsApi } from "@/api/clients";

export const clientsKey = (params) => ["clients", params || {}];
export const clientKey = (id) => ["client", id];

export function useClients(params) {
  return useQuery({
    queryKey: clientsKey(params),
    queryFn: () => clientsApi.list(params),
    placeholderData: keepPreviousData,
  });
}

// Split out for the file-size ratchet; re-exported for "@/hooks/useClients".
export { useClient } from "./useClientDetail";
export { useSendClientReminder } from "./useClientReceivables";
export {
  useCreateClient, useUpdateClient, useDeleteClient,
  useArchiveClient, useUnarchiveClient,
} from "./useClientMutations";
