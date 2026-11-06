import { Pencil, Printer, Trash2 } from "lucide-react";
import { useNavigate, useParams } from "react-router-dom";
import { Button } from "@/components/ui/Button";
import { InvoicePdfDownload } from "@/components/invoice/InvoicePdfDownload";
import { InvoiceSendEmailButton } from "@/components/invoice/InvoiceSendEmailButton";
import { useLang } from "@/context/LangContext";
import { printReceipt } from "@/lib/receiptDocument";

export function InvoiceDetailActions({ invoice, settings, canEdit, canDelete, onDelete }) {
  const nav = useNavigate();
  const { id } = useParams();
  const { t, lang } = useLang();
  const isPaid = invoice.effective_status === "paid";

  return (
    <div className="flex items-center gap-2 flex-wrap">
      {isPaid ? (
        <Button variant="outline" onClick={() => printReceipt({ invoice, settings, lang })}>
          <Printer size={15} /> {t("invoice.printReceipt")}
        </Button>
      ) : null}
      <InvoicePdfDownload invoice={invoice} settings={settings} lang={lang} label="PDF" />
      <InvoiceSendEmailButton invoice={invoice} />
      {canEdit && (
      <Button variant="outline" onClick={() => nav(`/invoices/${id}/edit`)}>
        <Pencil size={15} /> {t("common.edit")}
      </Button>
      )}
      {canDelete ? (
      <Button
        variant="ghost"
        onClick={onDelete}
        aria-label={t("common.delete")}
        title={t("common.delete")}
        className="text-[var(--danger)] hover:bg-[var(--danger)]/10"
      >
        <Trash2 size={15} />
      </Button>
      ) : null}
    </div>
  );
}