import { test } from "node:test";
import assert from "node:assert/strict";
import { register } from "node:module";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";

// node --test lacks Vite's resolution/JSX handling; see useInvoices.test.js.
const base = new URL("../", import.meta.url).href;
const nodeHook = `import { existsSync } from "node:fs";
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
}`;
register("data:text/javascript," + encodeURIComponent(nodeHook));

const { publicClientKey, publicClientInvoiceKey, usePublicClient } = await import("./usePublicClient.js");
const { publicClientApi } = await import("@/api/publicClient.js");

function renderHook(client, useHook) {
  let captured;
  function Probe() {
    captured = useHook();
    return null;
  }
  renderToStaticMarkup(createElement(QueryClientProvider, { client }, createElement(Probe)));
  return captured;
}

test("query keys are stable and token-scoped", () => {
  assert.deepEqual(publicClientKey("tok"), ["public-client", "tok"]);
  assert.deepEqual(publicClientInvoiceKey("tok", "inv"), ["public-client", "tok", "invoice", "inv"]);
});

test("usePublicClient fetches with the token and exposes the data", async () => {
  const qc = new QueryClient();
  const original = publicClientApi.get;
  let calledWith;
  publicClientApi.get = async (token) => {
    calledWith = token;
    return { client: { name: "Acme" }, invoices: [], payments: [] };
  };
  try {
    const query = renderHook(qc, () => usePublicClient("tok-1"));
    const data = await query.refetch();
    assert.equal(calledWith, "tok-1");
    assert.equal(data.data.client.name, "Acme");
  } finally {
    publicClientApi.get = original;
  }
});

test("usePublicClient does not fetch without a token", () => {
  const qc = new QueryClient();
  const original = publicClientApi.get;
  let called = false;
  publicClientApi.get = async () => {
    called = true;
    return {};
  };
  try {
    const query = renderHook(qc, () => usePublicClient(undefined));
    assert.equal(query.fetchStatus, "idle");
    assert.equal(called, false);
  } finally {
    publicClientApi.get = original;
  }
});
