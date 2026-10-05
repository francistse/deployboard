import { notFound } from "next/navigation";
import { getContent, isTranslatedLocale } from "@/content";
import { LandingView } from "../landing-view";

export const dynamicParams = false;

export function generateStaticParams() {
  return [{ locale: "ja" }, { locale: "zh-Hant" }, { locale: "zh-Hans" }];
}

export default async function LocalePage({
  params,
}: {
  params: Promise<{ locale: string }>;
}) {
  const { locale } = await params;
  if (!isTranslatedLocale(locale)) notFound();
  return <LandingView locale={locale} content={getContent(locale)} />;
}
