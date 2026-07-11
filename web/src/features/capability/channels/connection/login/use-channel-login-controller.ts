import { useCallback, useEffect, useState } from "react";

import {
  getChannelLoginApi,
  startChannelLoginApi,
  submitChannelLoginVerifyCodeApi,
  type ChannelLoginView,
  type ImChannelType,
} from "@/lib/api/capability/channel-api";
import { getErrorMessage } from "@/lib/error-message";

import type { ChannelPendingAction } from "../channel-connection-model";
import type { RunChannelCommand } from "../use-channel-command";
import { isChannelLoginRunning } from "./channel-login-model";

interface UseChannelLoginOptions {
  channelType: ImChannelType;
  enabled: boolean;
  onCompleted: () => Promise<void>;
  onError: (message: string) => void;
  pendingAction: ChannelPendingAction | null;
  runCommand: RunChannelCommand;
}

export function useChannelLoginController({
  channelType,
  enabled,
  onCompleted,
  onError,
  pendingAction,
  runCommand,
}: UseChannelLoginOptions) {
  const [view, setView] = useState<ChannelLoginView | null>(null);
  const running = isChannelLoginRunning(view);

  useEffect(() => {
    if (!enabled || !view?.login_id || !running) {
      return;
    }

    let disposed = false;
    let timer = 0;
    const poll = async () => {
      try {
        const nextView = await getChannelLoginApi(channelType, view.login_id);
        if (disposed) {
          return;
        }
        setView(nextView);
        if (nextView.status === "succeeded") {
          await onCompleted();
          return;
        }
        if (nextView.status === "running") {
          timer = window.setTimeout(poll, 1500);
        }
      } catch (error) {
        if (!disposed) {
          onError(getErrorMessage(error, "扫码登录状态刷新失败"));
        }
      }
    };

    timer = window.setTimeout(poll, 1500);
    return () => {
      disposed = true;
      window.clearTimeout(timer);
    };
  }, [channelType, enabled, onCompleted, onError, running, view?.login_id]);

  const startLogin = useCallback(async () => {
    if (!enabled) {
      return;
    }
    setView(await startChannelLoginApi(channelType));
  }, [channelType, enabled]);

  const submitVerifyCode = useCallback(async (value: string) => {
    if (!enabled || !view?.login_id) {
      return false;
    }
    const result = await runCommand({ kind: "verify-code" }, async () => {
      try {
        setView(await submitChannelLoginVerifyCodeApi(
          channelType,
          view.login_id,
          value,
        ));
        return true;
      } catch (error) {
        onError(getErrorMessage(error, "验证码提交失败"));
        return false;
      }
    });
    return result ?? false;
  }, [channelType, enabled, onError, runCommand, view?.login_id]);

  return {
    loading: pendingAction?.kind === "save"
      || pendingAction?.kind === "verify-code",
    running,
    startLogin,
    submitVerifyCode,
    view,
  };
}
