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

const { useEnsureClientPortal, useRevokeClientPortal } = await import("./useClientPortal.js");
const { clientKey } = await import("./useClients.js");
const { clientPortalApi } = await import("@/api/clientPortal.js");
const { LangProvider } = await import("@/context/LangContext");

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

test("useEnsureClientPortal calls ensure and invalidates the client key", async () => {
  const qc = new QueryClient();
  const original = clientPortalApi.ensure;
  let received;
  clientPortalApi.ensure = async (id) => {
    received = id;
    return { url_path: "/client/tok", token: "tok" };
  };
  try {
    let invalidated = false;
    const spy = qc.invalidateQueries.bind(qc);
    qc.invalidateQueries = (args) => {
      invalidated = true;
      return spy(args);
    };
    const mutation = renderHook(qc, useEnsureClientPortal);
    const data = await mutation.mutateAsync("c1");
    assert.equal(received, "c1");
    assert.equal(data.token, "tok");
    assert.equal(invalidated, true);
    assert.deepEqual(clientKey("c1"), ["client", "c1"]);
  } finally {
    clientPortalApi.ensure = original;
  }
});

test("useRevokeClientPortal surfaces failures without swallowing them", async () => {
  const qc = new QueryClient();
  const original = clientPortalApi.revoke;
  clientPortalApi.revoke = async () => {
    throw { status: 403, message: "forbidden" };
  };
  try {
    const mutation = renderHook(qc, useRevokeClientPortal);
    await assert.rejects(() => mutation.mutateAsync("c1"));
  } finally {
    clientPortalApi.revoke = original;
  }
});
