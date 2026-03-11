/**
 * 静态资源 URL 构造工具
 *
 * [INPUT]: 依赖 NEXT_PUBLIC_STATIC_ASSET_PREFIX 环境变量和资源原始路径
 * [OUTPUT]: 对外提供 buildStaticAssetUrl
 * [POS]: lib 模块的静态资源路径层，被 metadata/public 资源消费
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

const staticAssetPrefix = process.env.NEXT_PUBLIC_STATIC_ASSET_PREFIX?.replace(/\/+$/, "") || "";

export function buildStaticAssetUrl(path: string): string {
  if (!path) {
    return path;
  }

  const normalizedPath = path.startsWith("/") ? path : `/${path}`;
  if (!staticAssetPrefix) {
    return normalizedPath;
  }

  return `${staticAssetPrefix}${normalizedPath}`;
}
