"use client";

import { Navigate } from "react-router-dom";
import { LoaderCircle } from "lucide-react";

import { APP_ROUTE_PATHS, AppRouteBuilders } from "@/app/router/route-paths";
import { isDesktopRuntime } from "@/config/desktop-runtime";
import { canUseOperations } from "@/features/settings/operations/operations-access";
import { useAuth } from "@/shared/auth/auth-context";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";

export function OperationsPage() {
  const { loading, status } = useAuth();

  if (loading) {
    return (
      <div className="flex h-full items-center justify-center">
        <LoaderCircle
          aria-hidden="true"
          className={getUiSpinnerClassName({ size: "xl", tone: "primary" })}
        />
      </div>
    );
  }

  if (isDesktopRuntime() || !canUseOperations(status?.role)) {
    return <Navigate replace to={APP_ROUTE_PATHS.home} />;
  }

  return <Navigate replace to={AppRouteBuilders.settings("operations")} />;
}
