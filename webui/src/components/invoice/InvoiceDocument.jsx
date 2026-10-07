import {
  Document,
  Page,
  Text,
  View,
  Image,
  StyleSheet,
} from "@react-pdf/renderer";
import { formatMoney, formatDate } from "@/lib/utils";
import { t } from "@/lib/i18n";

const TEAL = "#0d9488";
const DARK = "#0c1a17";
const MUTED = "#5c7570";
const LINE = "#e2ece9";

const styles = StyleSheet.create({
  page: {
    padding: 40,
    fontSize: 10,
    color: DARK,
    fontFamily: "Helvetica",
  },
  headerRow: { flexDirection: "row", justifyContent: "space-between", alignItems: "flex-start" },
  logo: { width: 48, height: 48, objectFit: "contain", marginBottom: 8 },
  company: { fontSize: 15, fontFamily: "Helvetica-Bold", color: DARK },
  muted: { color: MUTED },
  invoiceTitle: { fontSize: 26, fontFamily: "Helvetica-Bold", color: TEAL, letterSpacing: 1, textTransform: "uppercase" },
  badge: {
    marginTop: 6,
    alignSelf: "flex-end",
    paddingVertical: 3,
    paddingHorizontal: 8,
    borderRadius: 10,
    backgroundColor: "#d3f4ec",
    color: TEAL,
    fontSize: 9,
    fontFamily: "Helvetica-Bold",
    textTransform: "uppercase",
  },
  metaRow: { flexDirection: "row", justifyContent: "space-between", marginTop: 28 },
  metaBlock: { maxWidth: 220 },
  label: { fontSize: 8, color: MUTED, textTransform: "uppercase", letterSpacing: 1, marginBottom: 4 },
  strong: { fontFamily: "Helvetica-Bold" },
  table: { marginTop: 28 },
  tHead: {
    flexDirection: "row",
    borderBottomWidth: 1,
    borderBottomColor: DARK,
    paddingBottom: 6,
  },
  tRow: {
    flexDirection: "row",
    paddingVertical: 7,
    borderBottomWidth: 1,
    borderBottomColor: LINE,
  },
  cDesc: { flex: 1 },
  cQty: { width: 50, textAlign: "right" },
  cRate: { width: 70, textAlign: "right" },
  cAmt: { width: 80, textAlign: "right" },
  totals: { marginTop: 16, marginLeft: "auto", width: 220 },
  totalRow: { flexDirection: "row", justifyContent: "space-between", paddingVertical: 3 },
  grandRow: {
    flexDirection: "row",
    justifyContent: "space-between",
    paddingVertical: 8,
    marginTop: 4,
    borderTopWidth: 1,
    borderTopColor: DARK,
  },
  grand: { fontSize: 14, fontFamily: "Helvetica-Bold", color: TEAL },
  notes: { marginTop: 30, paddingTop: 12, borderTopWidth: 1, borderTopColor: LINE },
  footer: { marginTop: 30, textAlign: "center", color: MUTED, fontSize: 8 },
});

// Renders each source line as its own block so multi-line notes/terms
// keep their line breaks in the PDF instead of collapsing to one line.
function Multiline({ text, style }) {
  return (
    <>
      {String(text ?? "").split("\n").map((line, i) => (
        <Text key={i} style={style}>{line === "" ? " " : line}</Text>
      ))}
    </>
  );
}

export function InvoiceDocument({ invoice, settings, lang = "en" }) {
  const s = settings || {};
  const currency = invoice.currency || "IDR";
  const statusLabel = t(lang, "status." + (invoice.effective_status || invoice.status || "draft")).toUpperCase();

  return (
    <Document title={invoice.invoice_number}>
      <Page size="A4" style={styles.page}>
        {/* Header */}
        <View style={styles.headerRow}>
          <View>
            {s.logo_url ? <Image src={s.logo_url} style={styles.logo} /> : null}
            <Text style={styles.company}>{s.company_name || "Your Company"}</Text>
            {s.address ? <Text style={styles.muted}>{s.address}</Text> : null}
            {s.email ? <Text style={styles.muted}>{s.email}</Text> : null}
            {s.phone ? <Text style={styles.muted}>{s.phone}</Text> : null}
          </View>
          <View style={{ alignItems: "flex-end" }}>
            <Text style={styles.invoiceTitle}>{t(lang, "common.invoice")}</Text>
            <Text style={[styles.muted, { marginTop: 4 }]}>{invoice.invoice_number}</Text>
            <Text style={styles.badge}>{statusLabel}</Text>
          </View>
        </View>

        {/* Meta */}
        <View style={styles.metaRow}>
          <View style={styles.metaBlock}>
            <Text style={styles.label}>{t(lang, "invDetail.billTo")}</Text>
            <Text style={styles.strong}>{invoice.client_name || "—"}</Text>
            {invoice.client_company ? <Text style={styles.muted}>{invoice.client_company}</Text> : null}
            {invoice.client_email ? <Text style={styles.muted}>{invoice.client_email}</Text> : null}
            {invoice.client_address ? <Text style={styles.muted}>{invoice.client_address}</Text> : null}
          </View>
          <View style={{ alignItems: "flex-end" }}>
            <View style={{ flexDirection: "row", marginBottom: 6 }}>
              <Text style={[styles.label, { width: 70, textAlign: "right", marginRight: 10 }]}>{t(lang, "invDetail.issued")}</Text>
              <Text style={{ width: 90, textAlign: "right" }}>{formatDate(invoice.issue_date)}</Text>
            </View>
            <View style={{ flexDirection: "row" }}>
              <Text style={[styles.label, { width: 70, textAlign: "right", marginRight: 10 }]}>{t(lang, "invDetail.due")}</Text>
              <Text style={{ width: 90, textAlign: "right" }}>{formatDate(invoice.due_date)}</Text>
            </View>
          </View>
        </View>

        {/* Line items */}
        <View style={styles.table}>
          <View style={styles.tHead}>
            <Text style={[styles.cDesc, styles.label]}>{t(lang, "common.description")}</Text>
            <Text style={[styles.cQty, styles.label]}>{t(lang, "common.qty")}</Text>
            <Text style={[styles.cRate, styles.label]}>{t(lang, "common.rate")}</Text>
            <Text style={[styles.cAmt, styles.label]}>{t(lang, "common.amount")}</Text>
          </View>
          {(invoice.items || []).map((it, i) => (
            <View style={styles.tRow} key={i}>
              <Text style={styles.cDesc}>{it.description || "—"}</Text>
              <Text style={styles.cQty}>{Number(it.quantity)}</Text>
              <Text style={styles.cRate}>{formatMoney(it.rate, currency)}</Text>
              <Text style={styles.cAmt}>{formatMoney(it.amount, currency)}</Text>
            </View>
          ))}
        </View>

        {/* Totals */}
        <View style={styles.totals}>
          <View style={styles.totalRow}>
            <Text style={styles.muted}>{t(lang, "common.subtotal")}</Text>
            <Text>{formatMoney(invoice.subtotal, currency)}</Text>
          </View>
          {Number(invoice.discount) > 0 && (
            <View style={styles.totalRow}>
              <Text style={styles.muted}>{t(lang, "common.discount")}</Text>
              <Text>− {formatMoney(invoice.discount, currency)}</Text>
            </View>
          )}
          <View style={styles.totalRow}>
            <Text style={styles.muted}>{t(lang, "invDetail.taxLine", { n: Number(invoice.tax_rate) })}</Text>
            <Text>{formatMoney(invoice.tax_amount, currency)}</Text>
          </View>
          <View style={styles.grandRow}>
            <Text style={styles.grand}>{t(lang, "common.total")}</Text>
            <Text style={styles.grand}>{formatMoney(invoice.total, currency)}</Text>
          </View>
        </View>

        {/* Notes */}
        {(invoice.notes || invoice.terms) && (
          <View style={styles.notes}>
            {invoice.notes ? (
              <>
                <Text style={styles.label}>{t(lang, "common.notes")}</Text>
                <Multiline text={invoice.notes} style={{ marginBottom: 8 }} />
              </>
            ) : null}
            {invoice.terms ? (
              <>
                <Text style={styles.label}>{t(lang, "common.terms")}</Text>
                <Multiline text={invoice.terms} />
              </>
            ) : null}
          </View>
        )}

        <Text style={styles.footer}>
          {t(lang, "common.thankYou")}
        </Text>
      </Page>
    </Document>
  );
}
