import { test } from "node:test";
import assert from "node:assert/strict";
import { register } from "node:module";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { MemoryRouter } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";

// Render the Login page under node --test with the same tiny resolver/loader as
// useInvoices.test.js: real JSX components, a stubbed auth context and a
// controllable config hook, so oidc_error from the URL drives the banner.
const base = new URL("../", import.meta.url).href;
const authStub =
  "data:text/javascript," +
  encodeURIComponent(
    "export const useAuth = () => ({ login: async () => {}, sessionExpired: false, setSessionExpired: () => {} });",
  );
const configStub =
  "data:text/javascript," +
  encodeURIComponent(
    "export const useAllowRegistration = () => false; export const useOidcEnabled = () => false;",
  );
const turnstileStub =
  "data:text/javascript," +
  encodeURIComponent("export default function Turnstile() { return null; }");
const nodeHook = `import { existsSync, readFileSync } from "node:fs";
import { execFileSync } from "node:child_process";
import { fileURLToPath } from "node:url";
const base = ${JSON.stringify(base)};
const stubs = {
  "@/context/AuthContext": ${JSON.stringify(authStub)},
  "@/hooks/useConfig": ${JSON.stringify(configStub)},
  "@/components/auth/Turnstile": ${JSON.stringify(turnstileStub)},
};
export async function resolve(specifier, context, nextResolve) {
  if (stubs[specifier]) return nextResolve(stubs[specifier], context);
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
      "const frag = (out.match(/Fragment_[a-z0-9]+/g) || [])[0];" +
      "const fragImport = frag ? 'import { Fragment as ' + frag + ' } from \\"react\\";' : '';" +
      "process.stdout.write(fragImport + imports + out);";
    const out = execFileSync("bun", ["-e", script], { input: src, encoding: "utf8" });
    return { format: "module", source: out, shortCircuit: true };
  }
  return nextLoad(url, context);
}`;
register("data:text/javascript," + encodeURIComponent(nodeHook));

const Login = (await import("@/pages/Login.jsx")).default;
const { LangProvider } = await import("@/context/LangContext");

function renderAt(path) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return renderToStaticMarkup(
    createElement(
      QueryClientProvider,
      { client },
      createElement(LangProvider, null, createElement(MemoryRouter, { initialEntries: [path] }, createElement(Login))),
    ),
  );
}

test("Login shows the localized SSO error for a known oidc_error code", () => {
  const html = renderAt("/login?oidc_error=state");
  assert.match(html, /expired|kedaluwarsa/i);
});

test("Login shows no SSO error without the query param", () => {
  const html = renderAt("/login");
  assert.doesNotMatch(html, /kedaluwarsa/i);
});

test("Login placeholders come from i18n, not hardcoded English", () => {
  const html = renderAt("/login");
  assert.match(html, /placeholder="Email"/);
  assert.match(html, /placeholder="Password"/);
  assert.doesNotMatch(html, /placeholder="email"/);
  assert.doesNotMatch(html, /placeholder="password"/);
});
