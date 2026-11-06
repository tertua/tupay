import { apiClient } from "./http";

// adminGatewayApi holds the relay project mutations. Key/secret rotation is
// step-up guarded: the request must carry a single-use X-Step-Up-Token minted
// by POST /auth/step-up.
export const adminGatewayApi = {
  deleteGatewayProject: (slug) =>
    apiClient.delete(`/admin/gateway/projects/${slug}`).then((r) => r.data),
  rotateGatewayKey: (slug, stepUpToken) =>
    apiClient
      .post(`/admin/gateway/projects/${slug}/rotate-key`, null, {
        headers: { "X-Step-Up-Token": stepUpToken },
      })
      .then((r) => r.data.project),
  rotateGatewaySecret: (slug, stepUpToken) =>
    apiClient
      .post(`/admin/gateway/projects/${slug}/rotate-secret`, null, {
        headers: { "X-Step-Up-Token": stepUpToken },
      })
      .then((r) => r.data.project),
};
