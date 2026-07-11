"use client";

import { Navigate } from "react-router-dom";

import { APP_ROUTE_PATHS, AppRouteBuilders } from "@/app/router/route-paths";
import { isDesktopRuntime } from "@/config/desktop-runtime";
import { canUseOperations } from "@/features/settings/operations/operations-access";
import { useAuth } from "@/shared/auth/auth-context";

export function OperationsPage() {
  const { loading, status } = useAuth();

  if (loading) {
    return (
      <div className="flex h-full items-center justify-center">
        <div className="h-6 w-6 animate-spin rounded-full border-2 border-primary border-t-transparent" />
      </div>
    );
  }

  if (isDesktopRuntime() || !canUseOperations(status?.role)) {
    return <Navigate replace to={APP_ROUTE_PATHS.home} />;
  }

  return <Navigate replace to={AppRouteBuilders.settings("operations")} />;
}
