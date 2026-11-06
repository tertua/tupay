// Auth-expiry policy for the axios client, split out of http.js so that file
// stays inside its size ratchet.

// 401s that are part of a normal unauthenticated flow — they must stay
// inline (e.g. wrong password) and never trigger a global logout.
const AUTH_EXPIRED_EXCLUSIONS = [
  "/auth/login",
  "/auth/register",
  "/auth/forgot-password",
  "/auth/reset-password",
  "/auth/me",
  "/auth/logout",
  "/auth/step-up",
  "/config",
  "/public/",
];

// Step-up refusals are 401s by design: the destructive call is rejected until
// the user re-confirms. They carry this code so the MPA opens the prompt
// instead of logging the user out.
export const STEP_UP_REQUIRED_CODE = "step_up_required";

export function isStepUpRefusal(err) {
  return err?.response?.data?.error?.details?.code === STEP_UP_REQUIRED_CODE;
}

export function shouldBroadcastExpired(url) {
  if (typeof url !== "string" || !url) return true;
  return !AUTH_EXPIRED_EXCLUSIONS.some((excluded) => url.includes(excluded));
}
