import { apiClient } from "./http";

// Sends one invoice to its client's billing email; enqueues an outbox mail
// server-side and resolves with { queued: true }. Resend is allowed.
export const invoiceSendEmailApi = {
  send: (id) => apiClient.post(`/invoices/${id}/send-email`, {}).then((r) => r.data),
};
