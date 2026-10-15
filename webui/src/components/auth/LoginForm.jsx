import { useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { motion } from "framer-motion";
import { ArrowRight, ArrowLeft, Loader2, Mail, Lock } from "lucide-react";
import { toast } from "sonner";
import { useQueryClient } from "@tanstack/react-query";

import { AuthField, AuthPrimaryButton, AuthErrorBanner } from "@/components/auth/AuthShell";
import Turnstile from "@/components/auth/Turnstile";
import { OidcButton } from "@/components/auth/OidcButton";
import AILogo from "@/components/layout/AILogo";
import { warmSessionAfterLogin } from "@/lib/sessionWarmup";
import { authVerifyApi } from "@/api/authVerify";
import { useLang } from "@/context/LangContext";
import { useAllowRegistration } from "@/hooks/useConfig";

// LoginForm owns the login form, the pending-verification resend affordance,
// and the post-login navigation. Split out of Login.jsx for the size ratchet.
export function LoginForm({ login, nav, location, sessionExpired, clearSessionExpired }) {
  const { t } = useLang();
  const allowRegistration = useAllowRegistration();
  const [searchParams] = useSearchParams();
  // A failed SSO round trip lands here as /login?oidc_error=<code>; an unknown
  // code falls back to the generic provider message.
  const oidcCode = searchParams.get("oidc_error");
  const oidcError = oidcCode ? t(`auth.oidcError.${oidcCode}`) : "";
  const [form, setForm] = useState({ email: "", password: "" });
  const [captchaToken, setCaptchaToken] = useState("");
  const [err, setErr] = useState("");
  const [loading, setLoading] = useState(false);
  const [resending, setResending] = useState(false);
  const [pendingVerification, setPendingVerification] = useState(false);
  const queryClient = useQueryClient();

  async function onSubmit(e) {
    e.preventDefault();
    setErr("");
    setPendingVerification(false);
    clearSessionExpired?.();
    setLoading(true);
    try {
      const minDelay = new Promise((r) => setTimeout(r, 800));
      await Promise.all([login(form, captchaToken).then(() => warmSessionAfterLogin(queryClient)), minDelay]);
      const destination = location.state?.from;
      nav(destination ? `${destination.pathname}${destination.search}${destination.hash}` : "/dashboard", { replace: true });
    } catch (e) {
      // The interceptor localizes the message, so the raw backend text (on the
      // original error) tells a pending account apart from a blocked one.
      const raw = e.original?.response?.data?.error?.message || "";
      setPendingVerification(e.status === 403 && raw === "account is pending verification");
      setErr(e.message || t("auth.loginFailed"));
    } finally {
      setLoading(false);
    }
  }

  async function onResend() {
    setResending(true);
    try {
      await authVerifyApi.resendVerification({ email: form.email });
      toast.success(t("auth.verify.resendSent"));
    } catch {
      toast.error(t("auth.verify.resendSent"));
    } finally {
      setResending(false);
    }
  }

  return (
    <motion.div
      initial={{ opacity: 0, y: 12 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.55, ease: [0.16, 1, 0.3, 1] }}
    >
      <div className="mb-12">
        <AILogo size={48} />
      </div>

      <h1 className="font-display text-[34px] font-semibold tracking-tight text-[var(--ink)] leading-[1.05]">
        {t("auth.welcomeBack")}
      </h1>
      <p className="text-[var(--ink-muted)] mt-2 text-[15px]">
        {t("auth.loginDesc")}
      </p>

      <form onSubmit={onSubmit} className="mt-9 space-y-4" autoComplete="off">
        <AuthField
          label={t("common.email")}
          type="email"
          autoComplete="email"
          value={form.email}
          onChange={(v) => setForm((current) => ({ ...current, email: v }))}
          icon={Mail}
          placeholder={t("common.email")}
        />

        <AuthField
          label={t("common.password")}
          type="password"
          autoComplete="current-password"
          value={form.password}
          onChange={(v) => setForm((current) => ({ ...current, password: v }))}
          icon={Lock}
          placeholder={t("common.password")}
          extra={
            <Link to="/forgot-password" className="text-xs text-[var(--accent-strong)] font-semibold hover:underline">
              {t("auth.forgot")}
            </Link>
          }
        />

        <AuthErrorBanner>{err || oidcError}</AuthErrorBanner>
        {pendingVerification && (
          <button
            type="button"
            onClick={onResend}
            disabled={resending || !form.email}
            className="w-full text-xs font-semibold text-[var(--accent-strong)] bg-[var(--accent-soft)] rounded-2xl px-4 py-2.5 leading-snug hover:underline disabled:opacity-60"
          >
            {resending ? t("auth.verify.resending") : t("auth.verify.resend")}
          </button>
        )}
        {sessionExpired && !err && (
          <div className="text-xs text-[var(--warning)] bg-[var(--warning)]/10 rounded-2xl px-4 py-2.5 leading-snug">
            {t("auth.sessionExpired")}
          </div>
        )}

        <Turnstile onVerify={setCaptchaToken} />

        <div className="pt-1">
          <AuthPrimaryButton type="submit" disabled={loading}>
            {loading ? (
              <>
                <Loader2 size={15} className="animate-spin" />
                {t("auth.signingIn")}
              </>
            ) : (
              <>
                {t("auth.signIn")} <ArrowRight size={15} />
              </>
            )}
          </AuthPrimaryButton>
        </div>
      </form>

      <OidcButton />

      {allowRegistration && (
        <div className="text-sm text-[var(--ink-muted)] text-center mt-8">
          {t("auth.noAccount")}{" "}
          <Link to="/register" className="text-[var(--accent-strong)] font-semibold hover:underline">
            {t("auth.createOne")}
          </Link>
        </div>
      )}

      <div className="text-sm text-center mt-4">
        <Link
          to="/"
          className="inline-flex items-center gap-1.5 text-[var(--ink-muted)] font-medium hover:text-[var(--accent-strong)] hover:underline"
        >
          <ArrowLeft size={14} /> {t("auth.backToHome")}
        </Link>
      </div>
    </motion.div>
  );
}
