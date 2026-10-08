// Guard for the ID copy style rule in CODE_STYLE.md (Frontend → ❌ Don't):
// `id` is written the way an Indonesian user reads it, not exported from a
// translate tool. Familiar English terms stay as they are (`Scan struk`,
// `Dashboard`, `Gateway`); forced word-by-word KBBI equivalents are rejected.
// The list is deliberately short — only words that never belong in this UI.
import { test } from "node:test";
import assert from "node:assert/strict";
import { id } from "./i18n.id.js";
import { uiId } from "./i18n.id.ui.js";
import { settingsId } from "./i18n.id.settings.js";
import { gatewayId } from "./i18n.id.gateway.js";
import { clientsId } from "./i18n.id.clients.js";

const LITERAL_TRANSLATIONS = ["Pemindaian", "Dasbor", "Gerbang"];

function collectStrings(node, out = []) {
  if (typeof node === "string") {
    out.push(node);
  } else if (Array.isArray(node)) {
    for (const value of node) collectStrings(value, out);
  } else if (node && typeof node === "object") {
    for (const value of Object.values(node)) collectStrings(value, out);
  }
  return out;
}

test("ID copy has no literal translate-tool wording", () => {
  const strings = [id, uiId, settingsId, gatewayId, clientsId].flatMap((dict) => collectStrings(dict));
  assert.ok(strings.length > 0, "expected the ID dictionaries to yield strings");

  const hits = [];
  for (const value of strings) {
    for (const word of LITERAL_TRANSLATIONS) {
      if (new RegExp(`\\b${word}\\b`).test(value)) hits.push(`${word} → ${value}`);
    }
  }
  assert.deepEqual(hits, [], `literal ID wording:\n${hits.join("\n")}`);
});
