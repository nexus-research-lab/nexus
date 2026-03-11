import type { NextConfig } from "next";

const staticAssetPrefix = process.env.NEXT_PUBLIC_STATIC_ASSET_PREFIX?.replace(/\/+$/, "") || undefined;

const nextConfig: NextConfig = {
  reactStrictMode: false,
  output: "standalone",
  assetPrefix: staticAssetPrefix,
  turbopack: {
    root: process.cwd(),
  },
};

export default nextConfig;
