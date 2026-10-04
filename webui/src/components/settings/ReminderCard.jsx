import { Card, CardHeader, CardTitle, CardDescription } from "@/components/ui/Card";
import { Input } from "@/components/ui/Input";
import { Checkbox } from "@/components/ui/Checkbox";
import { useLang } from "@/context/LangContext";

function FieldLabel({ children }) {
  return <label className="text-xs font-medium text-[var(--ink-muted)] mb-1.5 block">{children}</label>;
}

// Automatic payment reminder card: an org-wide switch plus the two day windows
// around an invoice due date. 0 disables that leg.
export default function ReminderCard({ form, set, onToggleEnabled }) {
  const { t } = useLang();
  return (
    <Card padding="lg">
      <CardHeader>
        <div>
          <CardTitle className="text-base">{t("settings.reminders")}</CardTitle>
          <CardDescription className="mt-1">{t("settings.remindersDesc")}</CardDescription>
        </div>
      </CardHeader>
      <div className="space-y-4">
        <Checkbox
          checked={!!form.reminder_enabled}
          onChange={(v) => onToggleEnabled(v)}
          label={t("settings.reminderEnabled")}
        />
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <div>
            <FieldLabel>{t("settings.reminderBeforeDays")}</FieldLabel>
            <Input
              type="number"
              min="0"
              max="365"
              value={form.reminder_before_days}
              onChange={set("reminder_before_days")}
              className="tabular"
              placeholder="7"
            />
          </div>
          <div>
            <FieldLabel>{t("settings.reminderAfterDays")}</FieldLabel>
            <Input
              type="number"
              min="0"
              max="365"
              value={form.reminder_after_days}
              onChange={set("reminder_after_days")}
              className="tabular"
              placeholder="3"
            />
          </div>
        </div>
        <p className="text-[11px] text-[var(--ink-muted)]">{t("settings.reminderDaysHint")}</p>
      </div>
    </Card>
  );
}
