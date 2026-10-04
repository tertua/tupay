import DefaultsCard from "@/components/settings/DefaultsCard";
import ReminderCard from "@/components/settings/ReminderCard";

// Company-tab grouping of the invoicing defaults and the automatic payment
// reminder schedule. Kept as one wrapper so Settings.jsx renders a single
// element and stays within its size baseline.
export default function InvoicingDefaults({ form, set, selectClass, onToggleEnabled }) {
  return (
    <>
      <DefaultsCard form={form} set={set} selectClass={selectClass} />
      <ReminderCard form={form} set={set} onToggleEnabled={onToggleEnabled} />
    </>
  );
}
