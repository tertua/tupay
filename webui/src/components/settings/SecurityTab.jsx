import { useEffect, useState } from "react";
import { Loader2, ShieldCheck, ShieldOff } from "lucide-react";
import { toast } from "sonner";
import { Card, CardHeader, CardTitle, CardDescription } from "@/components/ui/Card";
import { Input } from "@/components/ui/Input";
import { Button } from "@/components/ui/Button";
import { authSecurityApi } from "@/api/authSecurity";
import { useLang } from "@/context/LangContext";

// SecurityTab owns the second-factor settings: enroll, confirm, and turn off.
export default function SecurityTab() {
  const { t } = useLang();
  const [status, setStatus] = useState(null);
  const [enrollment, setEnrollment] = useState(null);
  const [code, setCode] = useState("");
  const [password, setPassword] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    authSecurityApi
      .totpStatus()
      .then(setStatus)
      .catch(() => setStatus({ enabled: false }));
  }, []);

  if (!status) {
    return (
      <div className="flex items-center justify-center py-16 text-[var(--ink-muted)]">
        <Loader2 className="animate-spin" size={18} />
      </div>
    );
  }

  async function startEnroll() {
    setBusy(true);
    try {
      setEnrollment(await authSecurityApi.enrollTOTP());
    } catch (err) {
      if (err.status !== 401) toast.error(err.message || t("security.activateFailed"));
    } finally {
      setBusy(false);
    }
  }

  async function activate(e) {
    e.preventDefault();
    setBusy(true);
    try {
      await authSecurityApi.activateTOTP(code);
      setStatus({ enabled: true });
      setEnrollment(null);
      setCode("");
      toast.success(t("security.activated"));
    } catch (err) {
      if (err.status !== 401) toast.error(err.message || t("security.activateFailed"));
    } finally {
      setBusy(false);
    }
  }

  async function disable(e) {
    e.preventDefault();
    setBusy(true);
    try {
      await authSecurityApi.disableTOTP({ password, code });
      setStatus({ enabled: false });
      setPassword("");
      setCode("");
      toast.success(t("security.disabledToast"));
    } catch (err) {
      if (err.status !== 401) toast.error(err.message || t("security.disableFailed"));
    } finally {
      setBusy(false);
    }
  }

  return (
    <Card padding="lg" className="max-w-2xl">
      <CardHeader>
        <div>
          <CardTitle className="text-base flex items-center gap-2">
            {status.enabled ? <ShieldCheck size={16} /> : <ShieldOff size={16} />}
            {t("security.title")}
          </CardTitle>
          <CardDescription className="mt-1">{t("security.desc")}</CardDescription>
        </div>
        <span className="text-xs font-medium text-[var(--ink-muted)]">
          {status.enabled ? t("security.enabled") : t("security.disabled")}
        </span>
      </CardHeader>

      {status.enabled ? (
        <form onSubmit={disable} className="space-y-3 max-w-sm">
          <p className="text-sm text-[var(--ink-muted)]">{t("security.disableDesc")}</p>
          <Input type="password" value={password} onChange={(e) => setPassword(e.target.value)} placeholder={t("security.password")} autoComplete="current-password" />
          <Input value={code} onChange={(e) => setCode(e.target.value.replace(/\D/g, "").slice(0, 6))} placeholder={t("security.code")} inputMode="numeric" autoComplete="one-time-code" />
          <Button type="submit" variant="outline" disabled={busy || !password || code.length !== 6}>
            {t("security.disable")}
          </Button>
        </form>
      ) : enrollment ? (
        <form onSubmit={activate} className="space-y-3 max-w-sm">
          <p className="text-sm text-[var(--ink-muted)]">{t("security.enrollDesc")}</p>
          <div className="text-xs text-[var(--ink-muted)]">{t("security.secret")}</div>
          <code className="block rounded-xl bg-[var(--surface-2)] px-3 py-2 text-xs break-all">{enrollment.secret}</code>
          <Input value={code} onChange={(e) => setCode(e.target.value.replace(/\D/g, "").slice(0, 6))} placeholder={t("security.code")} inputMode="numeric" autoComplete="one-time-code" autoFocus />
          <Button type="submit" variant="accent" disabled={busy || code.length !== 6}>
            {t("security.activate")}
          </Button>
        </form>
      ) : (
        <Button variant="accent" onClick={startEnroll} disabled={busy}>
          {t("security.enroll")}
        </Button>
      )}
    </Card>
  );
}
