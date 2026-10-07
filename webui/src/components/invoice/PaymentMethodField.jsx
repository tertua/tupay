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
  return (
    <div>
      <Checkbox
        checked={online}
        disabled={disabled}
        onChange={(v) => onChange(v ? "Online" : "Cash")}
        label={t("invEditor.onlinePayment")}
      />
      <p className="text-[11px] text-[var(--ink-muted)] mt-1.5">
        {t("invEditor.onlinePaymentHint")}
      </p>
    </div>
  );
}
