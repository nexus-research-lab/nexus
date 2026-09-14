// INPUT: 一次域名请求、用户确认/取消与当前语言。
// OUTPUT: 精确一次 Promise 结算与不含文案的本地校验状态；卸载结算为取消。
// POS: Shopify 输入事务控制器；不启动授权或解释远端结果。
"use client";

import { useCallback, useEffect, useRef, useState } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { normalizeShopDomain } from "./shop-domain-model";

export type ShopDomainPromptState =
  | { kind: "closed" }
  | { error: "invalid" | null; kind: "open" };

const CLOSED_PROMPT_STATE: ShopDomainPromptState = { kind: "closed" };

export function useShopDomainPrompt() {
  const { t } = useI18n();
  const [state, setState] = useState<ShopDomainPromptState>(CLOSED_PROMPT_STATE);
  const resolverRef = useRef<((value: string | null) => void) | null>(null);

  const settle = useCallback((value: string | null) => {
    const resolve = resolverRef.current;
    resolverRef.current = null;
    setState(CLOSED_PROMPT_STATE);
    resolve?.(value);
  }, []);

  const request = useCallback((): Promise<string | null> => {
    if (resolverRef.current) {
      return Promise.reject(new Error(t("capability.shop_domain_pending")));
    }
    setState({ error: null, kind: "open" });
    return new Promise((resolve) => {
      resolverRef.current = resolve;
    });
  }, [t]);

  const confirm = useCallback((value: string) => {
    const shop = normalizeShopDomain(value);
    if (!shop) {
      setState({ error: "invalid", kind: "open" });
      return;
    }
    settle(shop);
  }, [settle]);

  const cancel = useCallback(() => settle(null), [settle]);

  // 页面卸载必须结算等待中的命令，避免遗留永不完成的连接事务。
  useEffect(() => () => {
    resolverRef.current?.(null);
    resolverRef.current = null;
  }, []);

  return {
    cancel,
    confirm,
    request,
    state,
  };
}
