import { test } from "node:test";
import assert from "node:assert/strict";
import { registerHooks } from "node:module";

// The lib files are written for Vite: they use the `@/` alias and extensionless
// relative imports (`./i18n`). The plain Node test runner resolves neither, so
// shim them here and load the module exactly as the app does in production.
const hasExt = (specifier) => /\.[cm]?js$/.test(specifier);
registerHooks({
  resolve(specifier, context, nextResolve) {
    if (specifier.startsWith("@/")) {
      const url = new URL(`../${specifier.slice(2)}`, import.meta.url).href;
      return nextResolve(hasExt(url) ? url : `${url}.js`, context);
    }
    if (specifier.startsWith(".") && !hasExt(specifier)) {
      return nextResolve(`${specifier}.js`, context);
    }
    return nextResolve(specifier, context);
  },
});

const { escapeHtml, receiptHtml } = await import("./receiptDocument.js");
const { formatMoney } = await import("./utils.js");

function paidInvoice() {
  return {
    invoice_number: "INV-2026-001",
    effective_status: "paid",
    currency: "IDR",
    issue_date: "2026-11-01",
    total: "1500000",
    paid_amount: "1500000",
    payment_method: "Cash",
    items: [{ description: "Design", quantity: 2, rate: "750000", amount: "1500000" }],
  };
}

const settings = { company_name: "Toko Uji", address: "Jl. Mawar 1", phone: "0812" };

test("receiptHtml contains invoice number and formatted total", () => {
  const html = receiptHtml({ invoice: paidInvoice(), settings });
  assert.equal(html.includes("INV-2026-001"), true);
  assert.equal(html.includes(formatMoney("1500000", "IDR")), true);
});

test("receiptHtml escapes user-controlled company name", () => {
  const html = receiptHtml({
    invoice: paidInvoice(),
    settings: { ...settings, company_name: "<script>alert(1)</script>" },
  });
  assert.equal(html.includes("<script>alert(1)</script>"), false);
  assert.equal(html.includes("&lt;script&gt;alert(1)&lt;/script&gt;"), true);
});

test("receiptHtml targets 58mm paper", () => {
  const html = receiptHtml({ invoice: paidInvoice(), settings });
  assert.equal(html.includes("size: 58mm"), true);
});

test("receiptHtml is empty for an unpaid invoice", () => {
  const html = receiptHtml({ invoice: { ...paidInvoice(), effective_status: "sent" }, settings });
  assert.equal(html, "");
});

test("escapeHtml escapes every HTML metacharacter", () => {
  assert.equal(escapeHtml('a & b < c > d "e" \'f\''), "a &amp; b &lt; c &gt; d &quot;e&quot; &#39;f&#39;");
});
