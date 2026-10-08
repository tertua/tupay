import { apiClient } from "./http";

// Client CRUD + lifecycle/receivables calls. Kept as the single source; the
// legacy path `clients.js` re-exports this so callers are unaffected.
export const clientsApi = {
  // list forwards q/status/sort/order/page/per_page and returns the full
  // {clients, meta} data object so the page can render pagination.
  list: (params) => apiClient.get("/clients", { params }).then((r) => r.data),
  get: (id, params) => apiClient.get(`/clients/${id}`, { params }).then((r) => r.data),
  create: (payload) => apiClient.post("/clients", payload).then((r) => r.data.client),
  update: (id, payload) => apiClient.patch(`/clients/${id}`, payload).then((r) => r.data.client),
  remove: (id) => apiClient.delete(`/clients/${id}`).then(() => undefined),
  archive: (id) => apiClient.patch(`/clients/${id}/archive`, {}).then((r) => r.data.client),
  unarchive: (id) => apiClient.patch(`/clients/${id}/unarchive`, {}).then((r) => r.data.client),
  sendReminder: (id) => apiClient.post(`/clients/${id}/reminder`, {}).then((r) => r.data),
  // statement downloads the CSV as a blob through the shared client (axios
  // stays confined to http.js), bypassing the JSON envelope on purpose.
  statement: (id, params) =>
    apiClient.get(`/clients/${id}/statement.csv`, { params, responseType: "blob" }).then((r) => r.data),
};
