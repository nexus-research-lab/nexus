/**
 * INPUT: Composer 草稿中的本地 File。
 * OUTPUT: 可用于图片预览的 Object URL，并在替换或卸载时释放。
 * POS: Composer 本地附件预览的浏览器资源生命周期边界。
 */
"use client";

import { useEffect, useState } from "react";

export function useComposerLocalFileUrl(file: File): string | null {
  const [fileUrl, setFileUrl] = useState<string | null>(null);

  useEffect(() => {
    const nextFileUrl = URL.createObjectURL(file);
    setFileUrl(nextFileUrl);
    return () => {
      URL.revokeObjectURL(nextFileUrl);
    };
  }, [file]);

  return fileUrl;
}
