"use client";

import { useCallback, useEffect, useRef, useState } from "react";

import {
  normalizeShopDomain,
  SHOP_DOMAIN_VALIDATION_MESSAGE,
} from "./shop-domain-model";

export type ShopDomainPromptState =
  | { kind: "closed" }
  | { error: string | null; kind: "open" };

const CLOSED_PROMPT_STATE: ShopDomainPromptState = { kind: "closed" };

export function useShopDomainPrompt() {
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
      return Promise.reject(new Error("店铺域名输入请求仍在处理中"));
    }
    setState({ error: null, kind: "open" });
    return new Promise((resolve) => {
      resolverRef.current = resolve;
    });
  }, []);

  const confirm = useCallback((value: string) => {
    const shop = normalizeShopDomain(value);
    if (!shop) {
      setState({ error: SHOP_DOMAIN_VALIDATION_MESSAGE, kind: "open" });
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
