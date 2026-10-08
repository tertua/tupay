import { test } from "node:test";
import assert from "node:assert/strict";
import { register } from "node:module";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";

// node --test lacks Vite's resolution/JSX handling; see useInvoices.test.js.
const base = new URL("../", import.meta.url).href;
const nodeHook = `import { existsSync, readFileSync } from "node:fs";
import { execFileSync } from "node:child_process";
import { fileURLToPath } from "node:url";
const base = ${JSON.stringify(base)};
export async function resolve(specifier, context, nextResolve) {
  if (specifier.startsWith("@/")) {
    let rel = specifier.slice(2);
    if (!/\\.[a-z]+$/.test(rel)) {
      for (const ext of [".js", ".jsx"]) {
        const url = new URL(rel + ext, base).href;
        if (existsSync(fileURLToPath(url))) return nextResolve(url, context);
      }
      rel += ".js";
    }
    return nextResolve(new URL(rel, base).href, context);
  }
  if (/^\\.\\.?\\//.test(specifier) && !/\\.[a-z]+$/.test(specifier) && context.parentURL) {
    for (const ext of [".js", ".jsx"]) {
      const url = new URL(specifier + ext, context.parentURL).href;
      if (existsSync(fileURLToPath(url))) return nextResolve(url, context);
    }
  }
  return nextResolve(specifier, context);
}
export async function load(url, context, nextLoad) {
  if (url.endsWith(".jsx")) {
    const src = readFileSync(fileURLToPath(url), "utf8");
    const script = "const out = new Bun.Transpiler({loader:'jsx'}).transformSync(await new Response(Bun.stdin.stream()).text());" +
      "const names = [...new Set(out.match(/jsx[A-Za-z]*_[a-z0-9]+/g) || [])];" +
      "const imports = names.map((n) => 'import { jsxDEV as ' + n + ' } from \\"react/jsx-dev-runtime\\";').join('');" +
      "process.stdout.write(imports + out);";
    const out = execFileSync("bun", ["-e", script], { input: src, encoding: "utf8" });
    return { format: "module", source: out, shortCircuit: true };
  }
  return nextLoad(url, context);
}`;
register("data:text/javascript," + encodeURIComponent(nodeHook));

const {
  subscriptionsKey,
  subscriptionKey,
  useSetSubscriptionStatus,
  useDeleteSubscription,
} = await import("./useSubscriptions.js");
const { subscriptionsApi } = await import("@/api/subscriptions.js");
const { LangProvider } = await import("@/context/LangContext");
const { en } = await import("@/lib/i18n.en.js");

function renderHook(client, useHook) {
  let captured;
  function Probe() {
    captured = useHook();
    return null;
  }
  renderToStaticMarkup(
    createElement(QueryClientProvider, { client }, createElement(LangProvider, null, createElement(Probe))),
  );
  return captured;
}

test("subscriptionsKey / subscriptionKey keep their cache shape", () => {
  assert.deepEqual(subscriptionsKey(), ["subscriptions", {}]);
  assert.deepEqual(subscriptionKey("t1"), ["subscription", "t1"]);
});

test("useSetSubscriptionStatus patches status and invalidates the list + detail", async () => {
  const qc = new QueryClient();
  const original = subscriptionsApi.setStatus;
  let received;
  subscriptionsApi.setStatus = async (id, status) => {
    received = { id, status };
    return status;
  };
  try {
    const keys = [];
    const spy = qc.invalidateQueries.bind(qc);
    qc.invalidateQueries = (args) => {
      keys.push(args.queryKey);
      return spy(args);
    };
    const m = renderHook(qc, useSetSubscriptionStatus);
    const data = await m.mutateAsync({ id: "t1", status: "paused" });
    assert.deepEqual(received, { id: "t1", status: "paused" });
    assert.equal(data, "paused");
    assert.ok(keys.some((k) => k[0] === "subscriptions"), "list invalidated");
    assert.deepEqual(keys.find((k) => k[0] === "subscription"), ["subscription", "t1"]);
  } finally {
    subscriptionsApi.setStatus = original;
  }
});

test("useDeleteSubscription surfaces failures without swallowing them", async () => {
  const qc = new QueryClient();
  const original = subscriptionsApi.remove;
  subscriptionsApi.remove = async () => {
    throw { status: 403, message: "forbidden" };
  };
  try {
    const m = renderHook(qc, useDeleteSubscription);
    await assert.rejects(() => m.mutateAsync("t1"));
  } finally {
    subscriptionsApi.remove = original;
  }
});

test("toast copy exists for the new subscription keys", () => {
  for (const key of ["subscriptions.statusFailed", "subscriptions.deleteFailed"]) {
    assert.notEqual(en[key], undefined, key);
  }
});
