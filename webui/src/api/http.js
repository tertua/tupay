import axios from "axios";
import { localizeApiError } from "@/lib/utils";
import { broadcastCaptchaReset, shouldResetCaptcha } from "./captchaReset";
import { isStepUpRefusal, shouldBroadcastExpired } from "./authExpiry";

// Broadcast when an authenticated call fails with 401 so the app can
// drop the stale user and bounce to /login without a manual reload.
// AuthContext listens for this; ProtectedShell then redirects.
export const AUTH_EXPIRED_EVENT = "tupay:auth-expired";

export function broadcastAuthExpired() {
  try {
    window.dispatchEvent(new CustomEvent(AUTH_EXPIRED_EVENT));
  } catch {
    /* non-browser / dispatch unsupported — ProtectedShell still guards routes */
  }
}

export const apiClient = axios.create({
  baseURL: "/api/v1",
  withCredentials: true,
  headers: { "Content-Type": "application/json" },
});

// CSRF double-submit: mirror the csrf_token cookie as its header on every
// request. The backend only enforces it for session-cookie mutations;
// public and API-key calls are unaffected.
function csrfToken() {
  try {
    const m = document.cookie.match(/(?:^|; )csrf_token=([^;]*)/);
    return m ? decodeURIComponent(m[1]) : "";
  } catch {
    return "";
  }
}

apiClient.interceptors.request.use((config) => {
  config.headers = config.headers ?? {};
  const token = csrfToken();
  if (token) config.headers["X-CSRF-Token"] = token;
  // UI language for endpoints that generate text (AI): same localStorage
  // key LangContext persists, allowlisted so a stale value can't leak through.
  config.headers["X-Locale"] = appLocale();
  return config;
});

// Mirrors LangContext's persisted language ("arr-lang").
function appLocale() {
  try {
    return localStorage.getItem("arr-lang") === "id" ? "id" : "en";
  } catch {
    return "en";
  }
}

apiClient.interceptors.response.use(
  (res) => res,
  (err) => {
    const status = err.response?.status;
    const stepUpRefusal = isStepUpRefusal(err);
    if (status === 401 && !stepUpRefusal && shouldBroadcastExpired(err.config?.url)) {
      broadcastAuthExpired();
    }
    if (shouldResetCaptcha(err.config?.url)) broadcastCaptchaReset();
    const message = localizeApiError(
      err.response?.data?.error?.message ||
        err.message ||
        "Request failed"
    );
    return Promise.reject({
      status,
      message,
      details: err.response?.data?.error?.details,
      stepUpRequired: stepUpRefusal,
      original: err,
    });
  }
);
