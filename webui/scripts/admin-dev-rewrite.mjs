// Dev serves files by name, so clean /admin URLs must rewrite to admin.html to match the production fallback (pkg/routes/webui_route.go).
export default function adminDevRewrite() {
  return {
    name: "admin-dev-rewrite",
    configureServer(server) {
      server.middlewares.use((req, _res, next) => {
        const url = req.url || "";
        if (url === "/admin" || url.startsWith("/admin/") || url.startsWith("/admin?")) {
          req.url = "/admin.html";
        }
        next();
      });
    },
  };
}
