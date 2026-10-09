import { apiClient } from "./http";

// randomUUID is only defined in secure contexts (HTTPS/localhost); fall back
// to a random v4-shaped id so self-hosted HTTP deployments can still void.
export function idempotencyKey() {
  if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") {
    return crypto.randomUUID();
  }
  return "xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx".replace(/[xy]/g, (ch) => {
    const r = (Math.random() * 16) | 0;
    const v = ch === "x" ? r : (r & 0x3) | 0x8;
    return v.toString(16);
  });
}

export const paymentsApi = {
  list: () => apiClient.get("/payments").then((r) => r.data),
  create: (payload, key) => apiClient.post("/payments", payload, key ? { headers: { "Idempotency-Key": key } } : undefined).then((r) => r.data.payment),
  remove: (id, reason) =>
    apiClient
      .delete(`/payments/${id}`, {
        params: { reason },
        headers: { "Idempotency-Key": idempotencyKey() },
      })
      .then((r) => r.data?.payment),
  createOnlineLink: (invoiceId) => apiClient.post("/payments/online", { invoiceId }).then((r) => r.data),
};
