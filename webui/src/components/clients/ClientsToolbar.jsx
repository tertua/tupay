import { Search } from "lucide-react";
import { SearchInput } from "@/components/ui/Input";
import { useLang } from "@/context/LangContext";

// ClientsToolbar drives the server-side list: a debounced search box, a
// lifecycle status filter, and a sort selector.
export function ClientsToolbar({ search, onSearch, status, onStatus, sort, onSort }) {
  const { t } = useLang();

  const STATUS = [
    { value: "", label: t("clients.statusAll") },
    { value: "active", label: t("clients.statusActive") },
    { value: "archived", label: t("clients.statusArchived") },
  ];
  const SORTS = [
    { value: "created_at:desc", label: t("clients.sortCreated") },
    { value: "name:asc", label: t("clients.sortName") },
    { value: "total_billed:desc", label: t("clients.sortBilled") },
    { value: "outstanding:desc", label: t("clients.sortOutstanding") },
  ];

  const selectClass =
    "h-10 rounded-full border border-[var(--border)] bg-[var(--surface)] px-3 text-sm text-[var(--ink)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--accent)]/40";

  return (
    <div className="flex flex-col md:flex-row md:items-center gap-3 mb-5">
      <div className="md:w-[320px]">
        <SearchInput
          leftIcon={<Search size={16} />}
          placeholder={t("clients.searchPlaceholder")}
          value={search}
          onChange={(e) => onSearch(e.target.value)}
        />
      </div>
      <select
        className={selectClass}
        aria-label={t("clients.filterStatus")}
        value={status}
        onChange={(e) => onStatus(e.target.value)}
      >
        {STATUS.map((o) => (
          <option key={o.value} value={o.value}>
            {o.label}
          </option>
        ))}
      </select>
      <select
        className={selectClass}
        aria-label={t("clients.sort")}
        value={sort}
        onChange={(e) => onSort(e.target.value)}
      >
        {SORTS.map((o) => (
          <option key={o.value} value={o.value}>
            {o.label}
          </option>
        ))}
      </select>
    </div>
  );
}
