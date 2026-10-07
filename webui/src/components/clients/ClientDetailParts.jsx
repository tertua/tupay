import { Card } from "@/components/ui/Card";

// Presentational pieces split out of ClientDetail.jsx so that page stays
// within its size ratchet.
export function ChartFallback() {
  return <div className="h-[320px] rounded-3xl bg-[var(--surface-2)] animate-pulse" />;
}

export function ContactRow({ icon: Icon, value, href }) {
  if (!value) return null;
  const content = (
    <div className="flex items-center gap-3 text-sm">
      <div className="h-8 w-8 rounded-xl bg-[var(--surface-2)] text-[var(--ink-muted)] flex items-center justify-center shrink-0">
        <Icon size={14} />
      </div>
      <span className="text-[var(--ink)] truncate">{value}</span>
    </div>
  );
  return href ? (
    <a href={href} className="block hover:text-[var(--accent-strong)]">
      {content}
    </a>
  ) : (
    content
  );
}

export function MiniStat({ label, value, warn }) {
  return (
    <Card padding="lg">
      <div className="text-[10px] uppercase tracking-wider text-[var(--ink-muted)] font-semibold">{label}</div>
      <div
        className={`font-display text-2xl font-semibold tabular mt-1.5 truncate ${
          warn ? "text-[var(--warning)]" : "text-[var(--ink)]"
        }`}
      >
        {value}
      </div>
    </Card>
  );
}
