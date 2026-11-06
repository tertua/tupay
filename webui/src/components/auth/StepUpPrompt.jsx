import { useEffect, useState } from "react";
import { AnimatePresence, motion } from "framer-motion";
import { X, Loader2, ShieldCheck } from "lucide-react";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { authSecurityApi } from "@/api/authSecurity";
import { useLang } from "@/context/LangContext";

// StepUpPrompt re-confirms the signed-in user before one destructive action and
// hands the caller a single-use token. A password account may confirm with its
// password; when an authenticator is enrolled the code is required as well.
export function StepUpPrompt({ open, onClose, onConfirmed }) {
  const { t } = useLang();
  const [password, setPassword] = useState("");
  const [code, setCode] = useState("");
  const [err, setErr] = useState("");
  const [saving, setSaving] = useState(false);
  const [totpEnabled, setTotpEnabled] = useState(false);

  useEffect(() => {
    if (open) {
      setPassword("");
      setCode("");
      setErr("");
      setSaving(false);
      authSecurityApi
        .totpStatus()
        .then((status) => setTotpEnabled(Boolean(status?.enabled)))
        .catch(() => setTotpEnabled(false));
    }
  }, [open]);

  useEffect(() => {
    if (!open) return;
    function onKey(e) {
      if (e.key === "Escape") onClose();
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open, onClose]);

  async function onSubmit(e) {
    e.preventDefault();
    setSaving(true);
    setErr("");
    try {
      const payload = {};
      if (password) payload.password = password;
      if (code) payload.code = code;
      const { token } = await authSecurityApi.stepUp(payload);
      onConfirmed(token);
      onClose();
    } catch (ex) {
      if (ex.status !== 401) setErr(ex.message || t("security.stepUpFailed"));
    } finally {
      setSaving(false);
    }
  }

  const canSubmit = (password || code) && !saving;

  return (
    <AnimatePresence>
      {open && (
        <motion.div className="fixed inset-0 z-50 flex items-center justify-center p-4" initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }}>
          <div className="absolute inset-0 bg-[var(--ink)]/30 backdrop-blur-sm" onClick={onClose} />
          <motion.div
            initial={{ opacity: 0, y: 12, scale: 0.98 }}
            animate={{ opacity: 1, y: 0, scale: 1 }}
            exit={{ opacity: 0, y: 8, scale: 0.98 }}
            transition={{ duration: 0.2, ease: [0.16, 1, 0.3, 1] }}
            className="relative w-full max-w-[440px] rounded-3xl bg-[var(--surface)] border border-[var(--border)] shadow-hover p-6"
          >
            <form onSubmit={onSubmit}>
              <div className="flex items-center justify-between mb-4">
                <h3 className="font-display text-lg font-semibold tracking-tight flex items-center gap-2">
                  <ShieldCheck size={18} /> {t("security.stepUpTitle")}
                </h3>
                <button type="button" onClick={onClose} className="h-8 w-8 rounded-full flex items-center justify-center text-[var(--ink-muted)] hover:bg-[var(--surface-2)]"><X size={16} /></button>
              </div>
              <p className="text-sm text-[var(--ink-muted)] leading-snug mb-4">{t("security.stepUpDesc")}</p>

              <label className="block mb-3">
                <span className="block text-xs font-medium text-[var(--ink-muted)] mb-1.5">{t("security.stepUpPassword")}</span>
                <Input type="password" value={password} onChange={(e) => setPassword(e.target.value)} autoFocus autoComplete="current-password" />
              </label>

              <label className="block">
                <span className="block text-xs font-medium text-[var(--ink-muted)] mb-1.5">
                  {totpEnabled ? t("security.totpCode") : t("security.totpCodeOptional")}
                </span>
                <Input
                  value={code}
                  onChange={(e) => setCode(e.target.value.replace(/\D/g, "").slice(0, 6))}
                  inputMode="numeric"
                  autoComplete="one-time-code"
                  placeholder="000000"
                />
              </label>

              {err && <p className="text-sm text-[var(--danger)] mt-3">{err}</p>}
              <div className="flex items-center justify-end gap-2 mt-6">
                <Button type="button" variant="outline" onClick={onClose}>{t("common.cancel")}</Button>
                <Button type="submit" variant="accent" disabled={!canSubmit}>
                  {saving && <Loader2 size={14} className="animate-spin" />}
                  {t("security.stepUpConfirm")}
                </Button>
              </div>
            </form>
          </motion.div>
        </motion.div>
      )}
    </AnimatePresence>
  );
}
