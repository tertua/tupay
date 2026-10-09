import { apiClient } from "./http";

export const adminUsersApi = {
  updateUserStatus: (id, status) =>
    apiClient.patch(`/admin/users/${id}/status`, { status }).then((r) => r.data.user),
  deleteUser: (id) => apiClient.delete(`/admin/users/${id}`).then(() => undefined),
};
