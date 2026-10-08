import { apiClient } from "./http";

// Subscriptions: the schedule the outbox worker expands into real invoices.
// Reads are open to any org member; every write is owner-only, so the page
// surfaces 403s inline rather than hiding the affordances.
export const subscriptionsApi = {
  list: (params = {}) =>
    apiClient.get("/subscriptions", { params }).then((r) => r.data.subscriptions),
  // The detail envelope carries items separately; fold them onto the subscription
  // so callers get the same flat { ...subscription, items } shape the invoice GET uses.
  get: (id) =>
    apiClient
      .get(`/subscriptions/${id}`)
      .then((r) => ({ ...r.data.subscription, items: r.data.items })),
  create: (payload) =>
    apiClient.post("/subscriptions", payload).then((r) => r.data.subscription),
  update: (id, payload) =>
    apiClient.patch(`/subscriptions/${id}`, payload).then((r) => r.data.subscription),
  setStatus: (id, status) =>
    apiClient.patch(`/subscriptions/${id}/status`, { status }).then((r) => r.data.status),
  remove: (id) => apiClient.delete(`/subscriptions/${id}`).then(() => undefined),
};
