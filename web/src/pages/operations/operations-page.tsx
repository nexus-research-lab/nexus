"use client";

import { Navigate } from "react-router-dom";

import { APP_ROUTE_PATHS } from "@/app/router/route-paths";
import { isDesktopRuntime } from "@/config/desktop-runtime";
import { OperationsPanel } from "@/features/operations/operations-panel";
import { useAuth } from "@/shared/auth/auth-context";
import { WorkspacePageFrame } from "@/shared/ui/workspace/frame/workspace-page-frame";

function canUseOperations(role?: string | null): boolean {
  return role === "owner" || role === "admin";
}

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

  return (
    <WorkspacePageFrame contentPaddingClassName="p-0">
      <OperationsPanel />
    </WorkspacePageFrame>
  );
}
