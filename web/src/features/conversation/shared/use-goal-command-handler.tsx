"use client";

import { useCallback, useMemo, useState } from "react";
import type { ReactNode } from "react";

import { ConfirmDialog } from "@/shared/ui/dialog/confirm-dialog";

import {
  goal_replacement_candidate,
  parse_goal_command,
  run_goal_command,
  type GoalCreateCommand,
} from "./goal-command";

interface GoalCommandHandlerOptions {
  session_key: string | null;
  on_refresh: () => void;
}

interface PendingGoalReplacement {
  command: GoalCreateCommand;
  current_objective: string;
}

export function useGoalCommandHandler({
  session_key,
  on_refresh,
}: GoalCommandHandlerOptions): {
  try_handle_goal_command: (content: string) => Promise<boolean>;
  goal_command_dialog: ReactNode;
} {
  const [pending_replacement, set_pending_replacement] =
    useState<PendingGoalReplacement | null>(null);
  const [replace_error, set_replace_error] = useState<string | null>(null);

  const run_and_refresh = useCallback(
    async (command: GoalCreateCommand, replace_existing: boolean) => {
      if (!session_key) return;
      await run_goal_command(session_key, command, { replace_existing });
      set_replace_error(null);
      set_pending_replacement(null);
      on_refresh();
    },
    [on_refresh, session_key],
  );

  const try_handle_goal_command = useCallback(
    async (content: string) => {
      const command = parse_goal_command(content);
      if (command === null) {
        return false;
      }
      if (!session_key) {
        return true;
      }
      if (command.kind === "create") {
        const current = await goal_replacement_candidate(session_key, command);
        if (current !== null) {
          set_replace_error(null);
          set_pending_replacement({
            command,
            current_objective: current.objective,
          });
          return true;
        }
      }
      await run_goal_command(session_key, command);
      on_refresh();
      return true;
    },
    [on_refresh, session_key],
  );

  const confirm_replacement = useCallback(() => {
    if (!pending_replacement) return;
    void run_and_refresh(pending_replacement.command, true).catch((error) => {
      set_replace_error(error instanceof Error ? error.message : "Goal 替换失败");
    });
  }, [pending_replacement, run_and_refresh]);

  const cancel_replacement = useCallback(() => {
    set_pending_replacement(null);
    set_replace_error(null);
    on_refresh();
  }, [on_refresh]);

  const goal_command_dialog = useMemo(
    () => (
      <ConfirmDialog
        cancel_text="保留当前"
        confirm_text="替换"
        is_open={pending_replacement !== null}
        message={
          replace_error
            ? `替换失败：${replace_error}`
            : `当前 Goal：${pending_replacement?.current_objective ?? ""}。新 Goal：${pending_replacement?.command.objective ?? ""}`
        }
        title="替换当前 Goal?"
        variant="danger"
        on_cancel={cancel_replacement}
        on_confirm={confirm_replacement}
      />
    ),
    [cancel_replacement, confirm_replacement, pending_replacement, replace_error],
  );

  return { try_handle_goal_command, goal_command_dialog };
}
