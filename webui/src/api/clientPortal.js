import { apiClient } from "./http";

// Owner-only client-portal link lifecycle. The list/detail/client CRUD lives
// in clients.js; this is the shareable-link surface, kept separate so the
// clients module stays within its size ratchet.
export const clientPortalApi = {
  ensure: (id) => apiClient.patch(`/clients/${id}/portal`).then((r) => r.data.client_portal),
  regenerate: (id) => apiClient.post(`/clients/${id}/portal/regenerate`).then((r) => r.data.client_portal),
  revoke: (id) => apiClient.delete(`/clients/${id}/portal`).then(() => undefined),
};
