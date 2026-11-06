import { useEffect, useRef, useState } from "react";
import { Loader2, Copy, Check, Download } from "lucide-react";
import { downloadImage } from "@/lib/download";
import { t } from "@/lib/i18n";
import { cryptoLabel } from "@/lib/cryptoAssets";
import { formatMoney } from "@/lib/utils";
import { useCryptoIntent } from "@/hooks/useCryptoIntent";
import { useCryptoPayable } from "@/hooks/useCryptoPayable";
import { Button } from "@/components/ui/Button";
import CryptoAssetPicker from "./CryptoAssetPicker";

const DEFAULT_TIMEOUT_SECONDS = 910;
const COPIED_FEEDBACK_DURATION = 2000;
const QR_SIZE = 200;
const QR_MARGIN = 1;

function formatCountdown(left) {
  const m = String(Math.floor(left / 60)).padStart(2, "0");
  const s = String(left % 60).padStart(2, "0");
  return `${m}:${s}`;
}

function payErrorMessage(e, lang) {
  const code = e?.details?.code;
  if (code === "amount_below_minimum") return t(lang, "public.payAmountMinimum");
  if (code === "rate_limited" || e?.status === 429) return t(lang, "public.payRateLimited");
  return t(lang, "public.payCreateFailed");
}

// Dev path — on-page crypto widget (multi-asset)
//   [done]  select-then-create: the asset starts unchosen and the gateway
//           intent is only requested after the payer picks one and taps
//           "create deposit address" — mounting the widget charges nothing.
//           The chosen asset's own live minimum is preflighted before that
//           tap; QR, address, amount, countdown, retry on 429, localized
//           failure messages, and "change asset" returns to the picker
//           without touching the API
//   [later] show the network fee per asset before the request
// Seam: props {token, lang, onError} + useCryptoIntent() (one UUID per
// asset, so a retry of the same asset replays the same charge instead of
// opening a second one) + useCryptoPayable() (asset-specific minimum).
// End dev path
export default function CryptoWidget({ token, lang, onError, onBack }) {
  // "" = the payer has not picked an asset yet; nothing is requested then.
  const [asset, setAsset] = useState("");
  // select → loading → pay; nothing leaves the browser until "pay".
  const [stage, setStage] = useState("select");
  const [intent, setIntent] = useState(null);
  const [copied, setCopied] = useState(false);
  const copyTimer = useRef(null);
  const { createIntent } = useCryptoIntent();
  const { checking, payable, minAmount, minCurrency } = useCryptoPayable(token, asset, stage === "select");
  const [left, setLeft] = useState(DEFAULT_TIMEOUT_SECONDS);
  const [qrSrc, setQrSrc] = useState("");
  const [saving, setSaving] = useState(false);
  const requestRef = useRef(0);

  function qrFileName() {
    const id = String(intent?.payment_id || intent?.order_id || "").replace(/[^A-Za-z0-9-_]/g, "").slice(0, 48);
    return `${id || "crypto-qr"}.png`;
  }

  async function saveQr() {
    if (!qrSrc || saving) return;
    setSaving(true);
    const ok = await downloadImage(qrSrc, qrFileName());
    setSaving(false);
    if (!ok) window.open(qrSrc, "_blank", "noopener");
  }

  // In-flight responses become stale once the widget unmounts or the payer
  // backs out to the picker, so a late reply never repaints the panel.
  useEffect(() => () => {
    requestRef.current += 1;
    if (copyTimer.current) clearTimeout(copyTimer.current);
  }, []);

  async function startPayment() {
    if (stage === "loading" || !asset || checking || !payable) return;
    const run = requestRef.current + 1;
    requestRef.current = run;
    setStage("loading");
    setIntent(null);
    setQrSrc("");
    try {
      const res = await createIntent(token, asset);
      if (run !== requestRef.current) return;
      if (res.address) {
        setIntent(res);
        setStage("pay");
      } else if (res.payment_url) {
        window.location.href = res.payment_url;
      } else {
        setStage("select");
        onError(t(lang, "public.payNoAddress"));
      }
    } catch (e) {
      if (run !== requestRef.current) return;
      setStage("select");
      onError(payErrorMessage(e, lang));
    }
  }

  function changeAsset() {
    requestRef.current += 1;
    setIntent(null);
    setQrSrc("");
    setLeft(DEFAULT_TIMEOUT_SECONDS);
    setStage("select");
  }

  useEffect(() => {
    let mounted = true;
    if (intent?.address) {
      import("qrcode").then(({ default: QRCode }) => {
        if (!mounted) return;
        QRCode.toDataURL(intent.address, { margin: QR_MARGIN, width: QR_SIZE })
          .then(setQrSrc)
          .catch((err) => console.warn("QR generation failed:", err));
      });
    }
    return () => {
      mounted = false;
    };
  }, [intent]);

  useEffect(() => {
    if (!intent?.address) return undefined;
    const start = Date.now();
    const total = DEFAULT_TIMEOUT_SECONDS * 1000;
    const timer = setInterval(() => {
      const remaining = Math.max(0, Math.round((start + total - Date.now()) / 1000));
      setLeft(remaining);
      if (remaining <= 0) {
        clearInterval(timer);
      }
    }, 1000);
    return () => clearInterval(timer);
  }, [intent]);

  function copy() {
    if (!intent?.address) return;
    try {
      navigator.clipboard?.writeText(intent.address);
      setCopied(true);
      if (copyTimer.current) clearTimeout(copyTimer.current);
      copyTimer.current = setTimeout(() => setCopied(false), COPIED_FEEDBACK_DURATION);
    } catch (err) {
      console.warn("Clipboard write failed:", err);
    }
  }

  const amount = intent?.pay_amount || intent?.amount || "—";
  const currency = cryptoLabel(intent?.pay_currency || asset);

  return (
    <div className="mt-6 rounded-2xl border border-[var(--border)] bg-[var(--surface-2)]/40 p-4 space-y-4">
      <div className="flex items-center justify-between">
        <div className="text-xs font-semibold uppercase tracking-wider text-[var(--ink-muted)]">
          {t(lang, "public.cryptoTitle")}
        </div>
        {stage === "pay" ? (
          <div className="tabular text-xs font-semibold text-[var(--danger)]">
            {t(lang, "public.cryptoExpires")} {formatCountdown(left)}
          </div>
        ) : null}
      </div>

      {stage === "select" ? (
        <>
          <p className="text-[11px] leading-relaxed text-[var(--ink-muted)]">{t(lang, "public.cryptoSelectHint")}</p>
          <CryptoAssetPicker value={asset} onChange={setAsset} lang={lang} />
          {!payable ? (
            <p className="text-xs text-[var(--danger)]">
              {t(lang, "public.belowMinimum", { amount: formatMoney(minAmount, minCurrency || "USD") })}
            </p>
          ) : null}
          <Button variant="accent" className="w-full" disabled={!asset || checking || !payable} onClick={startPayment}>
            {checking ? (
              <Loader2 size={16} className="animate-spin" />
            ) : (
              <>
                {t(lang, "public.cryptoCreateAddress")}
                {asset ? ` · ${cryptoLabel(asset)}` : ""}
              </>
            )}
          </Button>
        </>
      ) : stage === "loading" ? (
        <div className="flex justify-center py-4">
          <Loader2 className="animate-spin text-[var(--accent-strong)]" size={20} />
        </div>
      ) : intent ? (
        <>
          <div className="flex flex-col items-center gap-3">
            {qrSrc ? (
              <img src={qrSrc} alt={intent.address} className="h-48 w-48 rounded-xl border border-[var(--border)] bg-white p-2" />
            ) : (
              <div className="flex h-48 items-center justify-center text-xs text-[var(--ink-muted)]">QR…</div>
            )}
            {qrSrc ? (
              <button
                type="button"
                onClick={saveQr}
                disabled={saving}
                className="inline-flex items-center gap-1.5 rounded-full border border-[var(--border)] bg-[var(--surface)] px-4 py-2 text-xs font-semibold text-[var(--ink)] transition-colors hover:bg-[var(--surface-2)] disabled:opacity-60"
              >
                {saving ? <Loader2 size={13} className="animate-spin" /> : <Download size={13} />}
                {t(lang, "public.downloadQr")}
              </button>
            ) : null}
          </div>

          <div className="rounded-xl border border-[var(--border)] bg-[var(--surface)] p-3 space-y-1.5">
            <div className="text-xs text-[var(--ink-muted)]">{t(lang, "public.cryptoAddress")}</div>
            <button
              type="button"
              onClick={copy}
              className="flex w-full items-center justify-between gap-3 rounded-lg border border-[var(--border)] px-3 py-2 text-left text-sm font-mono text-[var(--ink)] hover:bg-[var(--surface-2)]"
            >
              <span className="truncate">{intent.address}</span>
              {copied ? <Check size={15} className="shrink-0 text-[var(--success)]" /> : <Copy size={15} className="shrink-0 text-[var(--ink-muted)]" />}
            </button>
          </div>

          <div className="flex items-center justify-between rounded-xl border border-[var(--border)] bg-[var(--surface)] px-3 py-2.5">
            <span className="text-xs text-[var(--ink-muted)]">{t(lang, "public.cryptoAmount")}</span>
            <span className="tabular text-sm font-semibold text-[var(--ink)]">{amount} {currency}</span>
          </div>

          <p className="text-[11px] leading-relaxed text-[var(--ink-muted)]">
            {t(lang, "public.cryptoHint")}
          </p>

          <Button variant="outline" className="w-full" onClick={changeAsset}>
            {t(lang, "public.cryptoChangeAsset")}
          </Button>
        </>
      ) : null}
      {onBack ? <Button variant="ghost" className="w-full" onClick={onBack}>{t(lang, "common.back")}</Button> : null}
    </div>
  );
}
