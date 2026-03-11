/**
 * 浏览器持久化存储辅助
 *
 * [INPUT]: 依赖 zustand persist middleware
 * [OUTPUT]: 对外提供构建期安全的 JSON storage 工厂
 * [POS]: lib 模块的持久化辅助，避免 SSR/静态构建访问 localStorage
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

import { createJSONStorage, StateStorage } from 'zustand/middleware';

const memoryStorage = (): StateStorage => {
  const storage = new Map<string, string>();
  return {
    getItem: (name) => storage.get(name) ?? null,
    setItem: (name, value) => {
      storage.set(name, value);
    },
    removeItem: (name) => {
      storage.delete(name);
    },
  };
};

export const createBrowserJSONStorage = () => createJSONStorage(() => {
  if (typeof window !== 'undefined' && window.localStorage) {
    return window.localStorage;
  }
  return memoryStorage();
});
