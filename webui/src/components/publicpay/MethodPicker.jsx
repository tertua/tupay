import { useMemo, useState } from "react";
import { Bitcoin, Check, CreditCard, Landmark, Loader2, QrCode, ShieldCheck, Wallet } from "lucide-react";
import { Button } from "@/components/ui/Button";
import { t } from "@/lib/i18n";
import { formatMoney } from "@/lib/utils";

// Placeholder brand mark per provider-neutral method id (see
// gateway.MethodBankTransfer/MethodQRIS/... in platform/gateway).
// Logo seam — real brand assets go here WITHOUT touching the row layout:
//   1. drop files as webui/public/pay-logos/<id>.svg (id = m.id, e.g. qris.svg)
//   2. point the matching entry below at its public path, e.g. qris: "/pay-logos/qris.svg"
//   3. leave the rest null — rows keep rendering the lucide fallback icon.
// (Kept as a code comment on purpose: docs go stale, this travels with the UI.)
const PAY_LOGOS = { bank_transfer: null, qris: null, gopay: null, credit_card: null, crypto: null, other: null };
const METHOD_ICONS = {
  bank_transfer: Landmark,
  qris: QrCode,
  gopay: Wallet,
  credit_card: CreditCard,
  crypto: Bitcoin,
};

// MethodPicker is select-then-continue on purpose: tapping a row only marks
// it selected, the gateway intent is created solely by the Continue button.
// That one explicit tap is what stops an accidental touch from firing a
// charge. Rows carry each method's own charged amount/currency (already
// converted with the owner's manual rate), so a cross-currency invoice shows
// its real numbers before anything is created.
export default function MethodPicker({ methods, lang, onPick, pending, error }) {
  const ordered = useMemo(() => {
    // Domestic IDR methods first; the backend sorts by id, this is display only.
    const rank = (m) => (String(m.currency || "").toUpperCase() === "IDR" ? 0 : 1);
    return [...methods].sort((a, b) => rank(a) - rank(b) || String(a.id).localeCompare(String(b.id)));
  }, [methods]);
  const [selected, setSelected] = useState("");
  const chosen = ordered.find((m) => m.id === selected) || null;
  if (!ordered.length) return <p className="mt-6 text-sm text-[var(--ink-muted)]">{t(lang, "public.notPayable")}</p>;

  return (
    <div className="mt-6">
      <div className="text-xs font-semibold uppercase tracking-wider text-[var(--ink-muted)] mb-3">
        {t(lang, "public.chooseMethod")}
      </div>
      <div role="radiogroup" aria-label={t(lang, "public.chooseMethod")} className="space-y-2">
        {ordered.map((m) => {
          const active = m.id === selected;
          const blocked = !!m.below_minimum;
          const Icon = METHOD_ICONS[m.id] || Wallet;
          const logo = PAY_LOGOS[m.id] || null;
          return (
            <button
              key={m.id}
              type="button"
              role="radio"
              aria-checked={active}
              aria-disabled={blocked}
              onClick={() => { if (!blocked) setSelected(m.id); }}
              disabled={!!pending || blocked}
              className={`w-full flex items-center gap-3 rounded-2xl border px-4 py-3 text-left transition-colors disabled:opacity-60 ${
                blocked
                  ? "border-[var(--border)] bg-[var(--surface)] cursor-not-allowed"
                  : active
                  ? "border-[var(--accent)]/60 bg-[var(--accent-soft)]/40"
                  : "border-[var(--border)] bg-[var(--surface)] hover:border-[var(--accent)]/50 hover:bg-[var(--surface-2)]"
              }`}
            >
              <span className="h-9 w-9 shrink-0 rounded-xl bg-[var(--surface-2)] border border-[var(--border)] flex items-center justify-center overflow-hidden">
                {logo ? (
                  <img src={logo} alt="" aria-hidden className="h-5 w-5 object-contain" />
                ) : (
                  <Icon size={17} className="text-[var(--accent-strong)]" />
                )}
              </span>
              <span className="min-w-0 flex-1">
                <span className="block text-sm font-semibold text-[var(--ink)] truncate">{m.name}</span>
                <span className="block text-xs tabular text-[var(--ink-muted)]">
                  {blocked
                    ? t(lang, "public.belowMinimum", { amount: formatMoney(m.min_amount, m.currency) })
                    : formatMoney(m.amount, m.currency)}
                </span>
              </span>
              <span
                className={`h-5 w-5 shrink-0 rounded-full border flex items-center justify-center transition-colors ${
                  active ? "border-[var(--accent)] bg-[var(--accent)] text-white" : "border-[var(--border)] text-transparent"
                }`}
              >
                <Check size={13} strokeWidth={3} />
              </span>
            </button>
          );
        })}
      </div>
      <Button
        variant="accent"
        className="w-full mt-4"
        disabled={!chosen || !!pending}
        onClick={() => chosen && onPick(chosen.id)}
      >
        {pending ? (
          <Loader2 size={16} className="animate-spin" />
        ) : (
          t(lang, "public.continuePay")
        )}
        {!pending && chosen ? <span className="tabular">· {formatMoney(chosen.amount, chosen.currency)}</span> : null}
      </Button>
      {error ? <p className="text-xs text-[var(--danger)] mt-3 text-center">{error}</p> : null}
      <div className="flex items-center justify-center gap-1.5 mt-4 text-[11px] text-[var(--ink-muted)]">
        <ShieldCheck size={13} /> {t(lang, "public.secureBy")}
      </div>
    </div>
  );
}
