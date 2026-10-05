import type { Metadata } from "next";
import type { Content } from "./types";
import { content as en } from "./en";
import { content as ja } from "./ja";
import { content as zhHant } from "./zh-Hant";
import { content as zhHans } from "./zh-Hans";

export type {
  ApiEndpoint,
  AlertRule,
  Content,
  ContrastColumn,
  FaqItem,
  FeatureItem,
  FooterLink,
  KeeperItem,
  LogLine,
  NavItem,
  PanelCategory,
  PanelGroup,
  PanelJob,
  Stat,
  StatusDef,
  StepItem,
} from "./types";

export const LOCALES = ["en", "ja", "zh-Hant", "zh-Hans"] as const;
export type Locale = (typeof LOCALES)[number];

const TRANSLATED_LOCALES = ["ja", "zh-Hant", "zh-Hans"] as const;
export type TranslatedLocale = (typeof TRANSLATED_LOCALES)[number];

export function isTranslatedLocale(value: string): value is TranslatedLocale {
  return (TRANSLATED_LOCALES as readonly string[]).includes(value);
}

const BY_LOCALE: Record<Locale, Content> = {
  en,
  ja,
  "zh-Hant": zhHant,
  "zh-Hans": zhHans,
};

/** English copy. Existing `import { content } from "@/content"` keeps working. */
export const content = en;

export function getContent(locale: Locale): Content {
  return BY_LOCALE[locale];
}

const basePath = process.env.NEXT_PUBLIC_BASE_PATH ?? "";

function withBasePath(path: string): string {
  return `${basePath}${path}`;
}

export function languageAlternates(): NonNullable<Metadata["alternates"]> {
  return {
    languages: {
      en: withBasePath("/"),
      ja: withBasePath("/ja/"),
      "zh-Hant": withBasePath("/zh-Hant/"),
      "zh-Hans": withBasePath("/zh-Hans/"),
      "x-default": withBasePath("/"),
    },
  };
}
