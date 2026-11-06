// Thermal 58mm receipt for a PAID invoice. Plain HTML/CSS rendered into a new
// window — deliberately not @react-pdf/renderer (CI bundle boundary) and no
// network calls, so the receipt prints even offline.
import { t } from "./i18n.js";
import { formatDate, formatMoney } from "./utils.js";
import { paymentMethodLabel } from "./paymentLabels.js";

const ESCAPES = { "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" };

// Every field written into the receipt window is user-controlled (company
// name, client name, item text, invoice number), so escape on the way in.
export function escapeHtml(value) {
  return String(value ?? "").replace(/[&<>"']/g, (ch) => ESCAPES[ch]);
}

const RECEIPT_CSS = `
@page { size: 58mm auto; margin: 0; }
* { box-sizing: border-box; }
body { width: 58mm; margin: 0; padding: 2mm; font-family: ui-monospace, Menlo, monospace; font-size: 11px; line-height: 1.35; color: #000; background: #fff; -webkit-print-color-adjust: exact; print-color-adjust: exact; }
.center { text-align: center; }
.row { display: flex; justify-content: space-between; gap: 2mm; }
.row > span:last-child { text-align: right; white-space: nowrap; }
.wrap { word-break: break-word; overflow-wrap: anywhere; }
.big { font-weight: 700; }
.muted { font-size: 10px; }
.rule { border-top: 1px dashed #000; margin: 2mm 0; }
.item { margin-bottom: 1.2mm; }
`;

function line(label, value, className = "") {
  return `<div class="row ${className}"><span class="wrap">${escapeHtml(label)}</span><span>${escapeHtml(value)}</span></div>`;
}

function itemBlock(item, currency) {
  const name = escapeHtml(item.description || "—");
  const meta = `${escapeHtml(Number(item.quantity))} × ${escapeHtml(formatMoney(item.rate, currency))}`;
  return `<div class="item"><div class="wrap">${name}</div><div class="row"><span class="muted wrap">${meta}</span><span>${escapeHtml(formatMoney(item.amount, currency))}</span></div></div>`;
}

// receiptHtml returns a full standalone document, or "" when the invoice is
// not paid (defensive: the caller only renders the button for paid invoices).
export function receiptHtml({ invoice, settings, lang = "en" }) {
  if (!invoice || invoice.effective_status !== "paid") return "";
  const s = settings || {};
  const currency = invoice.currency || "IDR";
  const latest = (invoice.payments || [])[0] || {};
  const method = latest.method || invoice.payment_method || "";
  const paidOn = latest.paid_on || "";
  const parts = [];

  parts.push(`<div class="center big wrap">${escapeHtml(s.company_name || "Your Company")}</div>`);
  if (s.address) parts.push(`<div class="center muted wrap">${escapeHtml(s.address)}</div>`);
  if (s.phone) parts.push(`<div class="center muted">${escapeHtml(s.phone)}</div>`);
  parts.push('<div class="rule"></div>');
  parts.push(`<div class="center big">${escapeHtml(t(lang, "common.invoice").toUpperCase())}</div>`);
  parts.push(`<div class="center">${escapeHtml(invoice.invoice_number)}</div>`);
  parts.push(line(t(lang, "invDetail.issued"), formatDate(invoice.issue_date)));
  if (invoice.client_name) parts.push(line(t(lang, "invDetail.billTo"), invoice.client_name));
  parts.push('<div class="rule"></div>');
  for (const item of invoice.items || []) parts.push(itemBlock(item, currency));
  parts.push('<div class="rule"></div>');
  parts.push(line(t(lang, "common.total"), formatMoney(invoice.total, currency), "big"));
  if (Number(invoice.paid_amount) > 0) {
    parts.push(line(t(lang, "invDetail.paidAmount"), formatMoney(invoice.paid_amount, currency)));
  }
  if (method) parts.push(line(t(lang, "common.method"), paymentMethodLabel(method)));
  if (paidOn) parts.push(line(t(lang, "common.date"), formatDate(paidOn)));
  parts.push('<div class="rule"></div>');
  parts.push(`<div class="center muted wrap">${escapeHtml(t(lang, "common.thankYou"))}</div>`);

  return `<!doctype html><html lang="${escapeHtml(lang)}"><head><meta charset="utf-8" /><title>${escapeHtml(invoice.invoice_number)}</title><style>${RECEIPT_CSS}</style></head><body>${parts.join("")}</body></html>`;
}

// printReceipt opens a popup sized for the receipt and triggers the browser
// print dialog. Manual only: nothing auto-prints when a payment settles.
// A blocked popup (window.open === null) is a no-op, never a crash.
export function printReceipt({ invoice, settings, lang = "en" }) {
  const html = receiptHtml({ invoice, settings, lang });
  if (!html) return;
  const win = window.open("", "_blank", "width=380,height=700");
  if (!win) return;
  win.document.write(html);
  win.document.close();
  win.focus();
  win.print();
}
