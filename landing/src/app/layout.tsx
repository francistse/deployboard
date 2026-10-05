import type { Metadata } from "next";
import { content, languageAlternates } from "@/content";
import "./globals.css";

const basePath = process.env.NEXT_PUBLIC_BASE_PATH ?? "";

export const metadata: Metadata = {
  title: content.meta.title,
  description: content.meta.description,
  icons: {
    icon: [{ url: `${basePath}/favicon.svg`, type: "image/svg+xml" }],
  },
  alternates: languageAlternates(),
};

/**
 * English stays at `/` and the other locales live under `[locale]`.
 * Each route renders its own `<html lang>` — a shared root tag would stamp
 * `lang="en"` onto every static page.
 */
export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return children;
}
