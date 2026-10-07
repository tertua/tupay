import { Check } from "lucide-react";
import { cn } from "@/lib/utils";

export function Checkbox({ checked, onChange, className, label, disabled }) {
  return (
    <button
      type="button"
      role="checkbox"
      aria-checked={checked}
      disabled={disabled}
      onClick={() => onChange(!checked)}
      className={cn(
        "inline-flex items-center gap-2 select-none rounded-md focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--accent)]/40",
        disabled && "opacity-50 cursor-not-allowed",
        className
      )}
    >
      <span
        className={cn(
          "h-4 w-4 rounded-md border flex items-center justify-center transition-colors",
          checked
            ? "bg-[var(--accent)] border-transparent text-white"
            : "bg-[var(--surface)] border-[var(--border)] text-transparent"
        )}
      >
        <Check size={11} strokeWidth={3} />
      </span>
      {label && <span className="text-xs text-[var(--ink-muted)]">{label}</span>}
    </button>
  );
}
