import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  reactStrictMode: false,
  output: "standalone",
  turbopack: {
    root: process.cwd(),
  },
};

export default nextConfig;
