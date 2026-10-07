import { lazy } from "react";
import { Navigate, useParams } from "react-router-dom";
import RouteError from "@/components/ui/RouteError";
import { useAllowRegistration } from "@/hooks/useConfig";
import { useInviteToken } from "@/hooks/useInviteToken";

const Landing = lazy(() => import("@/pages/Landing"));
const Login = lazy(() => import("@/pages/Login"));
const Register = lazy(() => import("@/pages/Register"));
const ForgotPassword = lazy(() => import("@/pages/ForgotPassword"));
const ResetPassword = lazy(() => import("@/pages/ResetPassword"));
const VerifyEmail = lazy(() => import("@/pages/VerifyEmail"));
const PublicPay = lazy(() => import("@/pages/PublicPay"));
const ClientPortal = lazy(() => import("@/pages/ClientPortal"));
const PaymentFinish = lazy(() => import("@/pages/PaymentFinish"));

function RegisterRoute() {
  const allowRegistration = useAllowRegistration();
  const inviteToken = useInviteToken();
  // A valid invite reveals registration even when the install is closed; the token is verified server-side on submit.
  if (allowRegistration || inviteToken) return <Register />;
  return <Navigate to="/login" replace />;
}

// Org invite emails link to /invite/<token>; forward the token to the register page the form actually reads it from.
function InviteRedirect() {
  const { token } = useParams();
  return <Navigate to={"/register?invite_token=" + encodeURIComponent(token || "")} replace />;
}

// Standalone public routes (no session shell), split out of routes.jsx so that file stays within its ratchet.
export const publicRoutes = [
  { path: "/", element: <Landing />, errorElement: <RouteError /> },
  { path: "/login", element: <Login />, errorElement: <RouteError /> },
  { path: "/register", element: <RegisterRoute />, errorElement: <RouteError /> },
  { path: "/invite/:token", element: <InviteRedirect />, errorElement: <RouteError /> },
  { path: "/forgot-password", element: <ForgotPassword />, errorElement: <RouteError /> },
  { path: "/reset-password", element: <ResetPassword />, errorElement: <RouteError /> },
  { path: "/verify-email", element: <VerifyEmail />, errorElement: <RouteError /> },
  { path: "/pay/:token", element: <PublicPay />, errorElement: <RouteError /> },
  { path: "/client/:token", element: <ClientPortal />, errorElement: <RouteError /> },
  { path: "/payment/finish", element: <PaymentFinish />, errorElement: <RouteError /> },
];

// Exported separately so routes.jsx can keep it as the very last entry (route order unchanged).
export const fallbackRoute = { path: "*", element: <Navigate to="/" replace /> };
