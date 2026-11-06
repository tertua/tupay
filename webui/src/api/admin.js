import { apiClient } from "./http";

export const adminApi = {
  listUsers: () => apiClient.get("/admin/users").then((r) => r.data.users),
  updateUserRole: (id, role) =>
    apiClient.patch(`/admin/users/${id}/role`, { role }).then((r) => r.data.user),
  listGatewayProjects: () => apiClient.get("/admin/gateway/projects").then((r) => r.data.projects),
  createGatewayProject: (payload) =>
    apiClient.post("/admin/gateway/projects", payload).then((r) => r.data.project),
  updateGatewayProject: (slug, payload) =>
    apiClient.patch(`/admin/gateway/projects/${slug}`, payload).then((r) => r.data.project),
  listGatewayTransactions: (params) =>
    apiClient.get("/admin/gateway/transactions", { params }).then((r) => r.data),
  listGatewayDeliveries: () =>
    apiClient.get("/admin/gateway/deliveries").then((r) => r.data.deliveries),
  retryGatewayDelivery: (id) =>
    apiClient.post(`/admin/gateway/deliveries/${id}/retry`).then((r) => r.data.delivery),
};
