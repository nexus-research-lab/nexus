import { ReactNode } from "react";

import { cn } from "@/lib/utils";
import { AppStage } from "@/shared/ui/app-stage";

interface RouteScaffoldProps {
  badge: string;
  title: string;
  description: string;
  meta?: ReactNode;
  children?: ReactNode;
  class_name?: string;
}

export function RouteScaffold({
  badge,
  title,
  description,
  meta,
  children,
  class_name,
}: RouteScaffoldProps) {
  return (
    <AppStage>
      <div className="relative flex min-h-0 flex-1 flex-col px-4 py-4 sm:px-6 sm:py-6">
        <section
          className={cn(
            "workspace-shell relative flex min-h-0 flex-1 flex-col overflow-hidden rounded-[30px]",
            class_name,
          )}
        >
          <div className="border-b workspace-divider px-8 py-6">
            <p className="text-[11px] font-semibold uppercase tracking-[0.24em] text-slate-700/48">
              {badge}
            </p>
            <div className="mt-2 flex flex-wrap items-end justify-between gap-4">
              <div className="min-w-0">
                <h1 className="text-[32px] font-black tracking-[-0.05em] text-slate-950/90 sm:text-[42px]">
                  {title}
                </h1>
                <p className="mt-2 max-w-3xl text-sm leading-6 text-slate-700/58 sm:text-[15px]">
                  {description}
                </p>
              </div>
              {meta ? <div className="shrink-0">{meta}</div> : null}
            </div>
          </div>

          <div className="flex min-h-0 flex-1 flex-col p-4 sm:p-6">
            {children}
          </div>
        </section>
      </div>
    </AppStage>
  );
}
