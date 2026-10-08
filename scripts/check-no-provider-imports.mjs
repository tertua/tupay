// Guard against controllers/models/queries importing a provider package.
//
// ARCHITECTURE.md key decision #4: "add provider = implement gateway.Gateway +
// one Register in main.go". Controllers must talk to the registry
// (platform/gateway) only; importing platform/midtrans or platform/nowpayments
// re-couples them to one provider and breaks that promise.
//
// Policy: a file under app/ must not import a *provider* package under
// platform/. Cross-cutting infrastructure packages (database, cache, relay,
// storage, ...) are allowed — those are not providers. INFRA_PACKAGES lists
// the permitted platform imports; anything else under platform/ is treated as
// a provider and fails.
//
// ORDERING: this check became a REQUIRED CI gate at the end of Phase 2 of
// plans/2026-09-30-gateway-provider-pluggability.md. The two files that used to
// import a provider package (settings_methods.go, gateway_intent_controller.go)
// were rewired through the registry, so DEFERRED_UNTIL_PHASE_2 is now empty and
// the check hard-fails on ANY provider import under app/. Do NOT re-add a name
// to that list — remove the import instead.
//
// Usage: node scripts/check-no-provider-imports.mjs
import { readdir, readFile } from "node:fs/promises";
import path from "node:path";

const root = path.resolve(import.meta.dirname, "..");

// Non-provider platform packages a controller may legitimately import.
const INFRA_PACKAGES = new Set([
  "gateway", // the abstraction itself
  "database",
  "cache",
  "relay",
  "storage",
  "mail",
  "outbox", // mail/SSE/webhook dispatch + scheduled sweeps (infra, not a provider)
  "captcha",
  "events",
  "ai",
]);

// Files whose provider import Phase 2 removes. Frozen empty — see ORDERING.
const DEFERRED_UNTIL_PHASE_2 = new Set([]);

const IMPORT = /"github\.com\/tertua\/tupay\/platform\/([a-z0-9_]+)"/g;

async function listFiles(dir) {
  const out = [];
  for (const e of await readdir(path.join(root, dir), { withFileTypes: true })) {
    if (e.name === "node_modules" || e.name.startsWith(".")) continue;
    const rel = path.posix.join(dir, e.name);
    if (e.isDirectory()) out.push(...(await listFiles(rel)));
    else if (e.name.endsWith(".go")) out.push(rel);
  }
  return out;
}

const files = await listFiles("app");
const deferred = [];
const violations = [];
for (const file of files) {
  const content = await readFile(path.join(root, file), "utf8");
  const providers = [...content.matchAll(IMPORT)]
    .map((m) => m[1])
    .filter((p) => !INFRA_PACKAGES.has(p));
  if (!providers.length) continue;
  const detail = `imports platform/${providers.join(", platform/")}`;
  if (DEFERRED_UNTIL_PHASE_2.has(file)) {
    deferred.push(`${file} ${detail} (deferred to Phase 2)`);
    continue;
  }
  violations.push(`${file} ${detail} — use the gateway registry instead of a provider package`);
}

for (const d of deferred) console.warn(`warn (deferred): ${d}`);

if (violations.length) {
  for (const v of violations) console.error(v);
  console.error("\nRoute through platform/gateway (registry + capabilities), then re-run: node scripts/check-no-provider-imports.mjs");
  process.exit(1);
}
console.log("provider imports OK: no app/ file imports a provider package.");
