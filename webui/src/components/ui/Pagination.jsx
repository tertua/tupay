import { ChevronLeft, ChevronRight } from "lucide-react";
import { Button } from "@/components/ui/Button";
import { useLang } from "@/context/LangContext";

// Paginator renders prev/next controls plus a "page X of Y" label from the
// server meta envelope ({page, per_page, total, total_pages}).
export function Paginator({ meta, onPage }) {
  const { t } = useLang();
  if (!meta || meta.total_pages <= 1) return null;

  const page = meta.page || 1;
  const totalPages = meta.total_pages || 1;

  return (
    <div className="flex items-center justify-between gap-3 mt-5">
      <span className="text-xs text-[var(--ink-muted)] tabular">
        {t("common.pageOf", { page, total: totalPages })}
      </span>
      <div className="flex items-center gap-2">
        <Button
          variant="outline"
          size="sm"
          disabled={page <= 1}
          onClick={() => onPage(page - 1)}
        >
          <ChevronLeft size={14} /> {t("common.prev")}
        </Button>
        <Button
          variant="outline"
          size="sm"
          disabled={page >= totalPages}
          onClick={() => onPage(page + 1)}
        >
          {t("common.next")} <ChevronRight size={14} />
        </Button>
      </div>
    </div>
  );
}
