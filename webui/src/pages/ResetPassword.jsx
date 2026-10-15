import { useState } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { motion } from "framer-motion";
import { ArrowLeft, Loader2, Lock, CheckCircle2 } from "lucide-react";
import {
  AuthShell,
  AuthField,
  AuthPrimaryButton,
  AuthErrorBanner,
} from "@/components/auth/AuthShell";
import AILogo from "@/components/layout/AILogo";
import Turnstile from "@/components/auth/Turnstile";
import { authApi } from "@/api/auth";
import { useLang } from "@/context/LangContext";

export default function ResetPassword() {
  const { t } = useLang();
  const [params] = useSearchParams();
  const nav = useNavigate();
  const token = params.get("token") || "";
  const [form, setForm] = useState({ password: "", confirm: "" });
  const [captchaToken, setCaptchaToken] = useState("");
  const [err, setErr] = useState("");
  const [loading, setLoading] = useState(false);
  const [done, setDone] = useState(false);

  async function onSubmit(e) {
    e.preventDefault();
    setErr("");
    if (form.password !== form.confirm) return setErr(t("auth.confirmMismatch"));
    if (!token) return setErr(t("auth.linkInvalid"));
    setLoading(true);
    try {
      await authApi.resetPassword({ token, new_password: form.password }, captchaToken);
      setDone(true);
      setTimeout(() => nav("/login"), 1500);
    } catch (ex) {
      setErr(ex.message || t("auth.linkInvalid"));
    } finally {
      setLoading(false);
    }
  }

  return (
    <AuthShell
      headline={
        <>
          {t("auth.headline.login1")}
          <br />
          <em style={{ fontStyle: "italic" }}>{t("auth.headline.login2")}</em>
        </>
      }
      subhead={t("auth.login.subhead")}
    >
      <motion.div
        initial={{ opacity: 0, y: 12 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.55, ease: [0.16, 1, 0.3, 1] }}
      >
        <div className="mb-12">
          <AILogo size={48} />
        </div>

        <h1 className="font-display text-[34px] font-semibold tracking-tight text-[var(--ink)] leading-[1.05]">
          {t("auth.resetTitle")}
        </h1>
        <p className="text-[var(--ink-muted)] mt-2 text-[15px]">
          {t("auth.resetDesc")}
        </p>

        {done ? (
          <motion.div
            initial={{ opacity: 0, y: 8 }}
            animate={{ opacity: 1, y: 0 }}
            className="mt-9 flex items-start gap-3 text-sm bg-[var(--accent-soft)] text-[var(--accent-strong)] rounded-2xl px-4 py-3.5"
          >
            <CheckCircle2 size={18} className="shrink-0 mt-0.5" />
            <span>{t("auth.resetSuccess")}</span>
          </motion.div>
        ) : (
          <form onSubmit={onSubmit} className="mt-9 space-y-4">
            <AuthField
              label={t("auth.newPassword")}
              type="password"
              autoComplete="new-password"
              value={form.password}
              onChange={(v) => setForm({ ...form, password: v })}
              placeholder={t("auth.atLeast8")}
              minLength={8}
              icon={Lock}
            />

            <AuthField
              label={t("auth.confirmPassword")}
              type="password"
              autoComplete="new-password"
              value={form.confirm}
              onChange={(v) => setForm({ ...form, confirm: v })}
              placeholder={t("auth.confirmPassword")}
              icon={Lock}
            />

            <AuthErrorBanner>{err}</AuthErrorBanner>

            <Turnstile onVerify={setCaptchaToken} />

            <div className="pt-1">
              <AuthPrimaryButton type="submit" disabled={loading}>
                {loading ? (
                  <>
                    <Loader2 size={15} className="animate-spin" />
                    {t("auth.resetting")}
                  </>
                ) : (
                  t("auth.resetTitle")
                )}
              </AuthPrimaryButton>
            </div>
          </form>
        )}

        <div className="text-sm text-[var(--ink-muted)] text-center mt-8">
          <Link
            to="/login"
            className="inline-flex items-center gap-1.5 text-[var(--accent-strong)] font-semibold hover:underline"
          >
            <ArrowLeft size={14} /> {t("auth.backToLogin")}
          </Link>
        </div>
      </motion.div>
    </AuthShell>
  );
}
