import { useState } from "react";
import { Link2, Copy, RefreshCw, Trash2, Check } from "lucide-react";
import { Card, CardTitle } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { useLang } from "@/context/LangContext";
import { useEnsureClientPortal, useRegenerateClientPortal, useRevokeClientPortal } from "@/hooks/useClientPortal";

// Owner control for the client's public portal link: create, copy, regenerate
// and revoke. The raw token is shown once (from the mutation result); an
// existing link only offers copy once freshly minted.
export default function ClientPortalCard({ clientId }) {
  const { t } = useLang();
  const ensure = useEnsureClientPortal();
  const regenerate = useRegenerateClientPortal();
  const revoke = useRevokeClientPortal();
  const [token, setToken] = useState("");
  const [copied, setCopied] = useState(false);

  const absolute = token ? `${window.location.origin}/client/${token}` : "";

  async function onCreate() {
    const data = await ensure.mutateAsync(clientId);
    if (data?.token) setToken(data.token);
  }

  async function onRegenerate() {
    if (!window.confirm(t("clients.portalRegenHint"))) return;
    const data = await regenerate.mutateAsync(clientId);
    if (data?.token) setToken(data.token);
  }

  async function onRevoke() {
    if (!window.confirm(t("clients.portalRegenHint"))) return;
    await revoke.mutateAsync(clientId);
    setToken("");
  }

  async function onCopy() {
    if (!absolute) return;
    try {
      await navigator.clipboard.writeText(absolute);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 2000);
    } catch {
      /* clipboard unavailable — the link stays visible for manual copy */
    }
  }

  return (
    <Card padding="lg">
      <div className="flex items-center gap-2 mb-1">
        <Link2 size={15} className="text-[var(--accent-strong)]" />
        <CardTitle>{t("clients.portal")}</CardTitle>
      </div>
      <p className="text-xs text-[var(--ink-muted)] mb-4">{t("clients.portalDesc")}</p>

      {token ? (
        <div className="flex items-center gap-2 flex-wrap">
          <code className="flex-1 min-w-0 truncate rounded-lg border border-[var(--border)] bg-[var(--surface-2)] px-3 py-2 text-xs text-[var(--ink)]">
            {absolute}
          </code>
          <Button variant="soft" size="sm" onClick={onCopy}>
            {copied ? <Check size={14} /> : <Copy size={14} />}
            {copied ? t("clients.portalCopied") : t("clients.portalCopy")}
          </Button>
        </div>
      ) : (
        <Button variant="accent" size="sm" onClick={onCreate} disabled={ensure.isPending}>
          <Link2 size={14} /> {t("clients.portalCreate")}
        </Button>
      )}

      {token && (
        <div className="flex items-center gap-2 mt-3">
          <Button variant="outline" size="sm" onClick={onRegenerate} disabled={regenerate.isPending}>
            <RefreshCw size={14} /> {t("clients.portalRegenerate")}
          </Button>
          <Button
            variant="ghost"
            size="sm"
            onClick={onRevoke}
            disabled={revoke.isPending}
            className="text-[var(--danger)] hover:bg-[var(--danger)]/10"
          >
            <Trash2 size={14} /> {t("clients.portalRevoke")}
          </Button>
        </div>
      )}
    </Card>
  );
}
