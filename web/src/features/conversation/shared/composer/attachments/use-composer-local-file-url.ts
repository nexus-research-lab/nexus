/**
 * INPUT: Composer 草稿中的本地 File。
 * OUTPUT: 仅属于当前 File 的 Object URL；替换时同步清空，替换/卸载时释放旧资源。
 * POS: Composer 本地附件预览的浏览器资源生命周期边界。
 */
"use client";

import { useEffect } from "react";
import { useResettableState } from "@/shared/lib/react/use-resettable-state";

export function useComposerLocalFileUrl(file: File): string | null {
  const [fileUrl, setFileUrl] = useResettableState<string | null>(null, file);

  useEffect(() => {
    const nextFileUrl = URL.createObjectURL(file);
    setFileUrl(nextFileUrl);
    return () => {
      URL.revokeObjectURL(nextFileUrl);
    };
  }, [file, setFileUrl]);

  return fileUrl;
}
