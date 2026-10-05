/**
 * Compatibility shim. The copy lives in `./content/`. Resolvers that prefer
 * `content.ts` over the `content/` directory still land on the same module.
 */
export {
  content,
  getContent,
  isTranslatedLocale,
  languageAlternates,
  LOCALES,
} from "./content/index";

export type {
  AlertRule,
  ApiEndpoint,
  Content,
  ContrastColumn,
  FaqItem,
  FeatureItem,
  FooterLink,
  KeeperItem,
  Locale,
  LogLine,
  NavItem,
  PanelCategory,
  PanelGroup,
  PanelJob,
  Stat,
  StatusDef,
  StepItem,
  TranslatedLocale,
} from "./content/index";
