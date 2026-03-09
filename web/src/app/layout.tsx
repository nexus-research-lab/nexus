import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "Nexus",
  description: "Nexus Core",
  icons: {
    icon: "/logo.png",
    shortcut: "/logo.png",
    apple: "/logo.png",
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
