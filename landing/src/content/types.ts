import type { ServiceStatus } from "@/kit/tokens";

export type PanelCategory = "ours" | "other" | "noise";

/**
 * A row in the hero panel, shaped the way the product's job table renders it.
 * Machine facts only — everything a reader reads as prose (provenance labels,
 * group names, column headers) lives in the locale, keyed from here.
 */
export interface PanelJob {
  label: string;
  status: ServiceStatus;
  category: PanelCategory;
  /** Provenance key; the badge text is `livePanel.sourceLabels[source]`. */
  source: "listed" | "path" | "unclassified";
  pid: number | null;
  exit: number;
  runs: number;
  alert: boolean;
  uptime?: string;
  keepAlive?: boolean;
  churn?: "churn" | "storm";
  retired?: boolean;
  when?: string;
}

/** A user-defined group, mirroring the product's group header row. */
export interface PanelGroup {
  /** Name key; the text is `livePanel.groupNames[key]`. */
  key: "app" | "ops" | "infra";
  health: { running: number; error: number; disabled: number };
  total: number;
  rows: readonly PanelJob[];
}

export interface LogLine {
  ts: string;
  level: "info" | "warn" | "error" | "stream";
  service: string;
  body: string;
}

export interface FeatureItem {
  index: string;
  title: string;
  body: string;
  accent?: "brand" | "accent" | "warning";
}

export interface StepItem {
  index: string;
  title: string;
  body: string;
  command?: string;
}

export interface Stat {
  value: string;
  label: string;
  caption?: string;
}

export interface FaqItem {
  question: string;
  answer: string;
}

export interface NavItem {
  label: string;
  href: string;
}

export interface ContrastColumn {
  label: string;
  bullets: readonly [string, string, string, string, string];
}

export interface StatusDef {
  status: ServiceStatus;
  rule: string;
}

export interface KeeperItem {
  name: string;
  body: string;
}

export interface AlertRule {
  trigger: string;
  kind: string;
}

export interface ApiEndpoint {
  method: string;
  path: string;
  note: string;
}

export interface FooterLink {
  label: string;
  href: string;
}

export interface Content {
  meta: {
    title: string;
    description: string;
  };
  brand: {
    name: string;
    mark: string;
    tagline: string;
    installCommand: string;
    launchCommand: string;
    github: string;
    githubUrl: string;
    changelogUrl: string;
    upstreamUrl: string;
    version: string;
    license: string;
  };
  nav: readonly [NavItem, NavItem, NavItem, NavItem, NavItem];
  ui: {
    skipToContent: string;
    github: string;
    copy: string;
    copied: string;
    primaryNav: string;
    language: string;
    settings: string;
  };
  hero: {
    eyebrow: string;
    headline: readonly [string, string];
    body: string;
    primaryCta: { label: string; href: string };
    secondaryCta: { label: string; href: string };
    metaRow: readonly [
      { key: string; value: string },
      { key: string; value: string },
      { key: string; value: string },
    ];
  };
  livePanel: {
    title: string;
    liveBadge: string;
    appTagline: string;
    badges: {
      alerts: string;
      inventory: string;
      writeMode: string;
      url: string;
    };
    searchPlaceholder: string;
    noiseToggle: string;
    /** Category chip labels, in the product's order: All, Ours, Noise, Other. */
    filters: {
      all: string;
      ours: string;
      noise: string;
      other: string;
    };
    /** Appended to the Noise chip count, which is not part of the visible set. */
    hiddenSuffix: string;
    /**
     * Whole-machine counts, from `content/fixtures.ts`. The chips and the status
     * tabs count every launchd job the way the dashboard does — which is why the
     * table below them shows far fewer rows.
     */
    counts: {
      categories: { all: number; ours: number; noise: number; other: number };
      statuses: Record<"all" | ServiceStatus, number>;
    };
    columns: readonly [string, string, string, string, string];
    jobsSuffix: string;
    nextRunPrefix: string;
    retiredLabel: string;
    churnLabel: string;
    restartLoopLabel: string;
    actions: {
      restart: string;
      start: string;
      enable: string;
      logs: string;
    };
    sourceLabels: {
      listed: string;
      path: string;
      unclassified: string;
    };
    groupNames: {
      app: string;
      ops: string;
      infra: string;
    };
    alertsOn: string;
    alertsOff: string;
    events: string;
    streamType: string;
    push: string;
    groups: readonly [PanelGroup, PanelGroup, PanelGroup];
  };
  /**
   * The hero preview's own controls. The three scenes are the things a reader
   * has to see, not read: the inventory views, `/metrics`, and a Telegram alert.
   */
  preview: {
    views: string;
    metrics: string;
    telegram: string;
    hint: string;
    noiseHeading: string;
    noiseNote: string;
  };
  why: {
    eyebrow: string;
    headline: string;
    paragraphs: readonly [string, string];
    contrast: {
      before: ContrastColumn;
      after: ContrastColumn;
    };
  };
  statusSection: {
    eyebrow: string;
    title: string;
    body: string;
  };
  statusMeta: Record<ServiceStatus, string>;
  statuses: readonly [
    StatusDef,
    StatusDef,
    StatusDef,
    StatusDef,
    StatusDef,
    StatusDef,
    StatusDef,
  ];
  forkSection: {
    eyebrow: string;
    title: string;
    body: string;
    upstreamLabel: string;
    keepers: readonly [
      KeeperItem,
      KeeperItem,
      KeeperItem,
      KeeperItem,
      KeeperItem,
      KeeperItem,
    ];
  };
  forkAdditions: readonly [FeatureItem, FeatureItem, FeatureItem, FeatureItem];
  logStream: readonly [
    LogLine,
    LogLine,
    LogLine,
    LogLine,
    LogLine,
    LogLine,
    LogLine,
  ];
  stream: {
    eyebrow: string;
    headline: string;
    body: string;
    badges: readonly [string, string, string, string];
    endpoint: string;
    endpointStatus: string;
  };
  metrics: {
    eyebrow: string;
    headline: string;
    body: string;
    sampleLabel: string;
    queryLabel: string;
    footnoteBefore: string;
    footnoteCode: string;
    footnoteAfter: string;
    sample: readonly [
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
      string,
    ];
    query: readonly [string, string, string, string];
  };
  alerts: {
    eyebrow: string;
    headline: string;
    body: string;
    channel: string;
    triggerHeader: string;
    kindHeader: string;
    footer: string;
    message: readonly [string, string, string, string, string];
    rules: readonly [AlertRule, AlertRule, AlertRule, AlertRule, AlertRule];
  };
  api: {
    eyebrow: string;
    title: string;
    body: string;
    verb: string;
    path: string;
    description: string;
    endpoints: readonly [
      ApiEndpoint,
      ApiEndpoint,
      ApiEndpoint,
      ApiEndpoint,
      ApiEndpoint,
      ApiEndpoint,
      ApiEndpoint,
      ApiEndpoint,
      ApiEndpoint,
      ApiEndpoint,
      ApiEndpoint,
      ApiEndpoint,
    ];
  };
  steps: readonly [StepItem, StepItem, StepItem];
  stats: readonly [Stat, Stat, Stat, Stat];
  faqSection: {
    eyebrow: string;
    title: string;
    body: string;
  };
  faqs: readonly [FaqItem, FaqItem, FaqItem, FaqItem, FaqItem, FaqItem];
  install: {
    eyebrow: string;
    title: string;
    body: string;
  };
  cta: {
    eyebrow: string;
    headline: string;
    body: string;
    command: string;
  };
  footer: {
    copyline: string;
    links: readonly [FooterLink, FooterLink, FooterLink, FooterLink, FooterLink];
  };
}
