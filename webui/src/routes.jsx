import { Navigate, createBrowserRouter, useLocation } from "react-router-dom";
import { AppShell } from "@/components/layout/AppShell";
import RouteError from "@/components/ui/RouteError";
import { useAuth } from "@/context/AuthContext";
import { useLang } from "@/context/LangContext";
import { fallbackRoute, publicRoutes } from "@/routes/publicRoutes";
import { protectedChildren } from "@/routes/protectedRoutes";

function ProtectedShell() {
  const { user, loading } = useAuth();
  const { t } = useLang();
  const location = useLocation();
  if (loading) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-[var(--bg)] text-[var(--ink-muted)] text-sm">
        {t("common.loading")}
      </div>
    );
  }
  if (!user) return <Navigate to="/login" state={{ from: location }} replace />;
  return <AppShell />;
}

export const router = createBrowserRouter([
  ...publicRoutes,
  {
    path: "/",
    element: <ProtectedShell />,
    errorElement: <RouteError />,
    children: [
      ...protectedChildren,
    ],
  },
  fallbackRoute,
]);
