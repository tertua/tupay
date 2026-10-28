import { useState } from "react";
import { Check, Copy, KeyRound } from "lucide-react";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { useLang } from "@/context/LangContext";

export function AdminGatewayCredentials({ project, onClose }) {
  const { t } = useLang();
  const [copied, setCopied] = useState("");
  async function copy(value, name) {
    await navigator.clipboard?.writeText(value);
    setCopied(name);
    window.setTimeout(() => setCopied(""), 1400);
  }
  return (
    <Card className="mb-5 border-[var(--accent)]/30 bg-[var(--accent-soft)]/30">
      <div className="flex items-start gap-3">
        <KeyRound size={18} className="mt-0.5 text-[var(--accent-strong)]" />
        <div className="min-w-0 flex-1">
          <div className="font-semibold text-[var(--ink)]">{t("gateway.credentialsTitle")}</div>
          <p className="mt-1 text-sm text-[var(--ink-muted)]">{t("gateway.credentialsDesc")}</p>
          {[
            ["apiKey", project.api_key],
            ["webhookSecret", project.webhook_secret],
          ].filter(([, value]) => Boolean(value)).map(([name, value]) => (
            <div key={name} className="mt-3 flex items-center gap-2">
              <code className="min-w-0 flex-1 truncate rounded-xl bg-[var(--surface)] px-3 py-2 text-xs text-[var(--ink)]">{value}</code>
              <Button type="button" size="iconSm" variant="outline" onClick={() => copy(value, name)} aria-label={t("gateway.copy")}>
                {copied === name ? <Check size={13} /> : <Copy size={13} />}
              </Button>
            </div>
          ))}
          <Button type="button" size="sm" variant="soft" className="mt-4" onClick={onClose}>{t("common.done")}</Button>
        </div>
      </div>
    </Card>
  );
}
