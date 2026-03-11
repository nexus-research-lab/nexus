import type { Metadata } from "next";
import "./globals.css";
import { buildStaticAssetUrl } from "@/lib/static-asset";

export const metadata: Metadata = {
  title: "Nexus",
  description: "Nexus Core",
  icons: {
    icon: buildStaticAssetUrl("/logo.png"),
    shortcut: buildStaticAssetUrl("/logo.png"),
    apple: buildStaticAssetUrl("/logo.png"),
  },
};

export default function RootLayout(
  {
    children,
  }: Readonly<{
    children: React.ReactNode;
  }>) {
  return (
    <html lang="zh-CN">
    <body className="antialiased">
    {children}
    </body>
    </html>
  );
}
