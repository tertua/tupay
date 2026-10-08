import { Badge } from "@/components/ui/Badge";
import { useLang } from "@/context/LangContext";

// Maps a client lifecycle status → badge tone + label.
export const CLIENT_STATUS = {
  active: { tone: "success", labelKey: "clientStatus.active" },
  archived: { tone: "neutral", labelKey: "clientStatus.archived" },
};

export function ClientStatusBadge({ status, className }) {
  const { t } = useLang();
  const s = CLIENT_STATUS[status] || CLIENT_STATUS.active;
  return (
    <Badge tone={s.tone} className={className}>
      {t(s.labelKey)}
    </Badge>
  );
}
