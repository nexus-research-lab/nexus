"use client";

import { Activity, AlertTriangle, CheckCircle2, PauseCircle } from "lucide-react";

import { cn } from "@/lib/utils";
import type { Goal, GoalEvent } from "@/types/conversation/goal";
import type { GoalContinuationHold } from "./goal-continuation-hold";
import {
  goal_event_label,
  goal_run_state,
  type GoalRunTone,
} from "./goal-panel-model";

const GOAL_RUN_TONE_CLASS: Record<
  GoalRunTone,
  {
    box: string;
    icon: string;
    text: string;
  }
> = {
  active: {
    box: "border-emerald-500/20 bg-emerald-500/5",
    icon: "text-emerald-600 dark:text-emerald-300",
    text: "text-emerald-700 dark:text-emerald-300",
  },
  waiting: {
    box: "border-amber-500/20 bg-amber-500/5",
    icon: "text-amber-600 dark:text-amber-300",
    text: "text-amber-700 dark:text-amber-300",
  },
  stopped: {
    box: "border-destructive/20 bg-destructive/5",
    icon: "text-destructive",
    text: "text-destructive",
  },
  done: {
    box: "border-sky-500/20 bg-sky-500/5",
    icon: "text-sky-600 dark:text-sky-300",
    text: "text-sky-700 dark:text-sky-300",
  },
};

interface GoalRunStateLineProps {
  continuation_hold?: GoalContinuationHold | null;
  goal: Goal;
  is_generating: boolean;
  latest_event: GoalEvent | null;
}

function GoalRunIcon({ tone }: { tone: GoalRunTone }) {
  const class_name = cn("h-3.5 w-3.5 shrink-0", GOAL_RUN_TONE_CLASS[tone].icon);
  if (tone === "active") return <Activity className={class_name} />;
  if (tone === "waiting") return <PauseCircle className={class_name} />;
  if (tone === "done") return <CheckCircle2 className={class_name} />;
  return <AlertTriangle className={class_name} />;
}

export function GoalRunStateLine({
  continuation_hold = null,
  goal,
  is_generating,
  latest_event,
}: GoalRunStateLineProps) {
  const state = goal_run_state(goal, is_generating, continuation_hold);
  const tone = GOAL_RUN_TONE_CLASS[state.tone];

  return (
    <div
      className={cn(
        "mt-2 flex min-w-0 flex-wrap items-center gap-1.5 rounded-md border px-2 py-1.5 text-[11px]",
        tone.box,
      )}
    >
      <span className={cn("inline-flex shrink-0 items-center gap-1 font-medium", tone.text)}>
        <GoalRunIcon tone={state.tone} />
        {state.label}
      </span>
      <span className="min-w-0 flex-1 truncate text-muted-foreground">
        {state.detail}
      </span>
      {latest_event ? (
        <span className="max-w-full truncate rounded border border-border/50 bg-background/60 px-1.5 py-0.5 text-muted-foreground">
          {goal_event_label(latest_event)}
        </span>
      ) : null}
    </div>
  );
}
