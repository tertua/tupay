import { apiClient } from "./http";

// Recurring invoice templates: the schedule the outbox worker expands into real
// invoices. Reads are open to any org member; every write is owner-only, so the
// page surfaces 403s inline rather than hiding the affordances.
export const invoiceTemplatesApi = {
  list: (params = {}) =>
    apiClient.get("/invoice-templates", { params }).then((r) => r.data.invoice_templates),
  // The detail envelope carries items separately; fold them onto the template so
  // callers get the same flat { ...template, items } shape the invoice GET uses.
  get: (id) =>
    apiClient
      .get(`/invoice-templates/${id}`)
      .then((r) => ({ ...r.data.invoice_template, items: r.data.items })),
  create: (payload) =>
    apiClient.post("/invoice-templates", payload).then((r) => r.data.invoice_template),
  update: (id, payload) =>
    apiClient.patch(`/invoice-templates/${id}`, payload).then((r) => r.data.invoice_template),
  setStatus: (id, status) =>
    apiClient.patch(`/invoice-templates/${id}/status`, { status }).then((r) => r.data.status),
  remove: (id) => apiClient.delete(`/invoice-templates/${id}`).then(() => undefined),
};
