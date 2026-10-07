import { useLang } from "@/context/LangContext";
import { Checkbox } from "@/components/ui/Checkbox";

// payment_method is a two-state choice in the editor, not a routing input:
// "Online" opts the invoice into the auto-created public payment link on
// Save & send (payment_link_ensure.go) and hides the manual record-payment
// button; off writes "Cash" because the client pays offline. The backend enum
// keeps "Bank transfer" for existing rows, and RecordPaymentModal still
// offers all three when logging how the money actually arrived.
export function PaymentMethodField({ value, disabled, onChange }) {
  const { t } = useLang();
  const online = value === "Online";
  // Legacy rows may carry a method outside the editor's two states
  // ("Bank transfer"); show the stored value raw — method names are never
  // localized (see paymentMethods.js) — so it is never mistaken for Cash.
  // Toggling overwrites it to Cash/Online only on user action.
  const isLegacy = value && value !== "Online" && value !== "Cash";
  return (
    <div>
      <Checkbox
        checked={online}
        disabled={disabled}
        onChange={(v) => onChange(v ? "Online" : "Cash")}
        label={t("invEditor.onlinePayment")}
      />
      {isLegacy && (
        <p className="text-[11px] text-[var(--ink-muted)] mt-1.5">
          {t("common.method")}: {value}
        </p>
      )}
      <p className="text-[11px] text-[var(--ink-muted)] mt-1.5">
        {t("invEditor.onlinePaymentHint")}
      </p>
    </div>
  );
}
