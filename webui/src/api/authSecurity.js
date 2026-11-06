import { apiClient } from "./http";

// authSecurityApi holds the second-factor and step-up endpoints, kept out of
// auth.js so that file stays inside its size ratchet.
export const authSecurityApi = {
  totpStatus: () => apiClient.get("/auth/totp").then((r) => r.data.totp),
  enrollTOTP: () => apiClient.post("/auth/totp/enroll").then((r) => r.data.totp),
  activateTOTP: (code) => apiClient.post("/auth/totp/activate", { code }).then((r) => r.data.totp),
  disableTOTP: (payload) => apiClient.post("/auth/totp/disable", payload).then(() => undefined),
  stepUp: (payload) => apiClient.post("/auth/step-up", payload).then((r) => r.data.step_up),
};

// withStepUp sends one destructive request carrying a fresh single-use token.
export function withStepUp(request, token) {
  return request({ headers: { "X-Step-Up-Token": token } });
}
