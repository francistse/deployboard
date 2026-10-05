import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { getContent, isTranslatedLocale, languageAlternates } from "@/content";
import { Document } from "../document";

export async function generateMetadata({
  params,
}: {
  params: Promise<{ locale: string }>;
}): Promise<Metadata> {
  const { locale } = await params;
  if (!isTranslatedLocale(locale)) return {};
  const copy = getContent(locale);
  return {
    title: copy.meta.title,
    description: copy.meta.description,
    alternates: languageAlternates(),
  };
}

export default async function LocaleLayout({
  children,
  params,
}: {
  children: React.ReactNode;
  params: Promise<{ locale: string }>;
}) {
  const { locale } = await params;
  if (!isTranslatedLocale(locale)) notFound();
  return <Document lang={locale}>{children}</Document>;
}
