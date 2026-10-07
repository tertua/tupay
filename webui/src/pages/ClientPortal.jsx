import { useEffect } from "react";
import { useParams, useSearchParams } from "react-router-dom";
import { Loader2 } from "lucide-react";
import { Card, CardTitle } from "@/components/ui/Card";
import PublicShell from "@/components/publicpay/PublicShell";
import { PublicErrorCard } from "@/components/publicpay/PublicErrorCard";
import ClientInvoiceRow from "@/components/clientportal/ClientInvoiceRow";
import ClientPaymentHistory from "@/components/clientportal/ClientPaymentHistory";
import { usePublicClient } from "@/hooks/usePublicClient";
import { t } from "@/lib/i18n";
import { setLocale } from "@/lib/utils";

const LANG_STORAGE_KEY = "arr-lang";

// Language detection mirrors PublicPay.jsx: explicit ?lang= wins, then the
// stored preference, then the browser, defaulting to English.
function detectLang(searchParams) {
  const param = searchParams?.get("lang");
  if (param === "en" || param === "id") return param;
  if (typeof window !== "undefined") {
    const stored = localStorage.getItem(LANG_STORAGE_KEY);
    if (stored === "en" || stored === "id") return stored;
  }
  if (typeof navigator !== "undefined" && navigator.language?.toLowerCase().startsWith("id")) return "id";
  return "en";
}

// Standalone public page: one client's invoices and payment history behind a
// shareable token. No session shell; the payer routes into /pay/:token for
// any actual payment.
export default function ClientPortal() {
  const { token } = useParams();
  const [searchParams] = useSearchParams();
  const lang = detectLang(searchParams);
  const { data, isLoading, error } = usePublicClient(token);

  useEffect(() => {
    document.documentElement.setAttribute("lang", lang);
    setLocale(lang);
  }, [lang]);

  if (isLoading) {
    return (
      <PublicShell>
        <Loader2 className="animate-spin text-[var(--ink-muted)]" size={22} />
      </PublicShell>
    );
  }
  if (error) {
    return (
      <PublicShell>
        <PublicErrorCard lang={lang} status={error?.status} message={error?.message} />
      </PublicShell>
    );
  }

  const { client, invoices = [], payments = [], branding } = data || {};
  const currency = invoices[0]?.currency || "IDR";

  return (
    <PublicShell branding={branding}>
      <Card padding="lg" className="w-full max-w-[640px]">
        <div className="pb-5 border-b border-[var(--border)]">
          <div className="text-[10px] uppercase tracking-wider text-[var(--ink-muted)] font-semibold mb-1">
            {t(lang, "public.clientTitle")}
          </div>
          <div className="font-display text-xl font-semibold tracking-tight text-[var(--ink)]">
            {client?.name || "—"}
          </div>
          {client?.company && <div className="text-xs text-[var(--ink-muted)]">{client.company}</div>}
          <p className="text-xs text-[var(--ink-muted)] mt-2">{t(lang, "public.clientSubtitle")}</p>
        </div>

        <div className="pt-4">
          <CardTitle className="mb-1">{t(lang, "public.clientInvoices")}</CardTitle>
          {invoices.length === 0 ? (
            <p className="text-sm text-[var(--ink-muted)] py-4">{t(lang, "public.clientEmptyInvoices")}</p>
          ) : (
            <div className="divide-y divide-[var(--border)]">
              {invoices.map((inv) => (
                <ClientInvoiceRow key={inv.id} invoice={inv} branding={branding} lang={lang} token={token} />
              ))}
            </div>
          )}
        </div>

        <div className="pt-6 mt-4 border-t border-[var(--border)]">
          <CardTitle className="mb-1">{t(lang, "public.clientHistory")}</CardTitle>
          <ClientPaymentHistory payments={payments} currency={currency} lang={lang} />
        </div>
      </Card>
    </PublicShell>
  );
}
