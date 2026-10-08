import { useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { clientsApi } from "@/api/clients";
import { useLang } from "@/context/LangContext";
import { clientKey } from "./useClients";
import { restoreSnapshots, snapshotQueries } from "@/lib/optimistic";

function invalidate(qc, id) {
  qc.invalidateQueries({ queryKey: ["clients"] });
  qc.invalidateQueries({ queryKey: ["dashboard"] });
  if (id) qc.invalidateQueries({ queryKey: ["client", id] });
}

// The clients list cache is {clients, meta}; patch rows inside it and leave
// other cache shapes alone. Mirrors patchList in lib/optimistic but for the
// wrapped list envelope.
function patchClientList(qc, mapFn) {
  qc.setQueriesData({ queryKey: ["clients"] }, (old) =>
    old && Array.isArray(old.clients) ? { ...old, clients: mapFn(old.clients) } : old
  );
}

export function useCreateClient() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (payload) => clientsApi.create(payload),
    onSuccess: () => invalidate(qc),
  });
}

// ClientFormModal catches and shows inline errors, so updates stay silent.
export function useUpdateClient() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, payload }) => clientsApi.update(id, payload),
    onMutate: async ({ id, payload }) => {
      const prev = await snapshotQueries(qc, [["clients"], ["client", id]]);
      patchClientList(qc, (list) => list.map((c) => (c.id === id ? { ...c, ...payload } : c)));
      qc.setQueryData(clientKey(id), (old) => (old ? { ...old, client: { ...old.client, ...payload } } : old));
      return { prev };
    },
    onError: (_err, _vars, ctx) => {
      if (ctx?.prev) restoreSnapshots(qc, ctx.prev);
    },
    onSettled: (_c, _e, { id }) => invalidate(qc, id),
  });
}

// ClientDetail deletes without a catch, so the hook owns the toast.
export function useDeleteClient() {
  const qc = useQueryClient();
  const { t } = useLang();
  return useMutation({
    mutationFn: (id) => clientsApi.remove(id),
    onMutate: async (id) => {
      const prev = await snapshotQueries(qc, [["clients"]]);
      patchClientList(qc, (list) => list.filter((c) => c.id !== id));
      return { prev };
    },
    onError: (_err, id, ctx) => {
      if (ctx?.prev) restoreSnapshots(qc, ctx.prev);
      toast.error(t("clients.deleteFailed"), { id: `client-delete-${id}` });
    },
    onSettled: () => invalidate(qc),
  });
}

// useSetClientStatus is the shared archive/unarchive body: optimistic status
// flip across the list + detail, rollback on error.
function useSetClientStatus(archived, action, successKey) {
  const qc = useQueryClient();
  const { t } = useLang();
  return useMutation({
    mutationFn: (id) => (archived ? clientsApi.archive(id) : clientsApi.unarchive(id)),
    onMutate: async (id) => {
      const prev = await snapshotQueries(qc, [["clients"], ["client", id]]);
      const status = archived ? "archived" : "active";
      patchClientList(qc, (list) => list.map((c) => (c.id === id ? { ...c, status } : c)));
      qc.setQueryData(clientKey(id), (old) => (old ? { ...old, client: { ...old.client, status } } : old));
      return { prev };
    },
    onError: (_err, _id, ctx) => {
      if (ctx?.prev) restoreSnapshots(qc, ctx.prev);
      toast.error(t("clients.archiveFailed"), { id: `client-${action}` });
    },
    onSuccess: () => toast.success(t(successKey)),
    onSettled: (_c, _e, id) => invalidate(qc, id),
  });
}

export function useArchiveClient() {
  return useSetClientStatus(true, "archive", "clients.archivedToast");
}

export function useUnarchiveClient() {
  return useSetClientStatus(false, "unarchive", "clients.unarchivedToast");
}
