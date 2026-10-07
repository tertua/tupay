import { apiClient } from "./http";

// Public client portal: one shareable token exposes a client's invoices and
// payment history. Reads only — the payer routes into the existing /pay flow
// for any actual payment.
export const publicClientApi = {
  get: (token) => apiClient.get(`/public/client/${token}`).then((r) => r.data),
  invoice: (token, id) => apiClient.get(`/public/client/${token}/invoice/${id}`).then((r) => r.data),
};
