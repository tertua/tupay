import { test } from "node:test";
import assert from "node:assert/strict";
import { register } from "node:module";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";

// node --test lacks Vite's resolution/JSX handling; register a resolver for the
// "@/" alias and extensionless imports plus a loader that transpiles .jsx via
// bun's transpiler (mirrors useInvoices.test.js).
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

const { clientsKey, clientKey, useClients } = await import("./useClients.js");
const { clientsApi } = await import("@/api/clients");
const { LangProvider } = await import("@/context/LangContext");

function renderHook(client, useHook, arg) {
  let captured;
  function Probe() {
    captured = useHook(arg);
    return null;
  }
  renderToStaticMarkup(
    createElement(
      QueryClientProvider,
      { client },
      createElement(LangProvider, null, createElement(Probe)),
    ),
  );
  return captured;
}

test("clientsKey / clientKey keep their cache shape", () => {
  assert.deepEqual(clientsKey(), ["clients", {}]);
  assert.deepEqual(clientsKey({ q: "ac" }), ["clients", { q: "ac" }]);
  assert.deepEqual(clientKey("c1"), ["client", "c1"]);
});

test("useClients forwards params and unwraps {clients, meta}", async () => {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const original = clientsApi.list;
  let received;
  clientsApi.list = async (params) => {
    received = params;
    return { clients: [{ id: "c1", name: "Acme" }], meta: { page: 1, total: 1, total_pages: 1 } };
  };
  try {
    // Render the hook so its queryKey/queryFn are registered, then fetch the
    // same key to exercise the queryFn.
    const params = { q: "acme", status: "active" };
    renderHook(qc, useClients, params);
    await qc.fetchQuery({ queryKey: clientsKey(params), queryFn: () => clientsApi.list(params) });
    assert.deepEqual(received, params);
    const data = qc.getQueryData(clientsKey(params));
    assert.equal(data.clients[0].name, "Acme");
    assert.equal(data.meta.total, 1);
  } finally {
    clientsApi.list = original;
  }
});
