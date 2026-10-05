"use client";

import { Fragment, createContext, useContext, useState } from "react";
import Link from "next/link";
import { motion } from "framer-motion";
import { Card, StatusDot } from "@/kit";
import { colors, type ServiceStatus } from "@/kit/tokens";
import { cn } from "@/kit/utils/cn";
import type { Content, Locale, PanelJob } from "@/content";
import { noiseJobs } from "@/content/fixtures";

const fadeUp = {
  hidden: { opacity: 0, y: 16 },
  show: { opacity: 1, y: 0, transition: { duration: 0.45, ease: [0.22, 1, 0.36, 1] as const } },
};

const LANGUAGE_LINKS: readonly { locale: Locale; label: string; href: string }[] = [
  { locale: "en", label: "English", href: "/" },
  { locale: "ja", label: "日本語", href: "/ja/" },
  { locale: "zh-Hant", label: "繁體中文", href: "/zh-Hant/" },
  { locale: "zh-Hans", label: "简体中文", href: "/zh-Hans/" },
];

const basePath = process.env.NEXT_PUBLIC_BASE_PATH ?? "";

const SiteContext = createContext<{ content: Content; locale: Locale } | null>(null);

function useSite() {
  const value = useContext(SiteContext);
  if (!value) {
    throw new Error("Landing view is missing site content");
  }
  return value;
}

export function LandingView({ locale, content }: { locale: Locale; content: Content }) {
  return (
    <SiteContext.Provider value={{ locale, content }}>
      <a
        href="#main"
        className="sr-only focus:not-sr-only focus:absolute focus:left-4 focus:top-4 focus:z-50 focus:rounded focus:bg-[#22D3EE] focus:px-3 focus:py-1.5 focus:text-[#05070D] focus:text-sm"
      >
        {content.ui.skipToContent}
      </a>
      <SiteHeader />
      <main id="main" className="relative min-h-dvh bg-[#05070D] text-[#F4F6FB] antialiased">
        <GridBackdrop />
        <Hero />
        <WhySection />
        <StatusReference />
        <ForkAdditions />
        <MetricsSection />
        <AlertsSection />
        <ApiSurface />
        <InstallBlock />
        <FaqSection />
        <SiteFooter />
      </main>
    </SiteContext.Provider>
  );
}

function SiteHeader() {
  const { content, locale } = useSite();
  const homeHref = locale === "en" ? "/" : `/${locale}/`;
  return (
    <header className="sticky top-0 z-40 border-b border-white/[0.05] bg-[#05070D]/85 backdrop-blur-md">
      <div className="mx-auto flex min-h-14 w-full max-w-[1180px] flex-wrap items-center justify-between gap-x-4 gap-y-2 px-6 py-2">
        <Link href={homeHref} className="flex items-baseline gap-2 transition-opacity hover:opacity-80">
          <span aria-hidden className="text-[14px] text-[#67E8F9]">▸</span>
          <span className="text-[14px] font-medium tracking-tight">{content.brand.name}</span>
          <span className="ml-1 font-mono text-[11px] text-white/35">{content.brand.version}</span>
        </Link>
        <div className="flex flex-wrap items-center justify-end gap-x-5 gap-y-1">
          <LanguageSwitcher />
          <nav aria-label={content.ui.primaryNav} className="hidden items-center gap-6 text-[13px] text-white/60 md:flex">
            {content.nav.map((l) => (
              <a key={l.href} href={l.href} className="transition-colors hover:text-white">
                {l.label}
              </a>
            ))}
            <a
              href={content.brand.githubUrl}
              target="_blank"
              rel="noreferrer"
              className="transition-colors hover:text-white"
            >
              {content.ui.github}
            </a>
          </nav>
        </div>
      </div>
    </header>
  );
}

function LanguageSwitcher() {
  const { content, locale } = useSite();
  return (
    <nav aria-label={content.ui.language} className="flex flex-wrap items-center gap-x-2.5 gap-y-1 text-[11px]">
      {LANGUAGE_LINKS.map((item) => {
        const active = item.locale === locale;
        return (
          <a
            key={item.locale}
            href={`${basePath}${item.href}`}
            hrefLang={item.locale}
            aria-current={active ? "page" : undefined}
            className={cn(
              "whitespace-nowrap transition-colors hover:text-white",
              active ? "text-[#67E8F9]" : "text-white/45",
            )}
          >
            {item.label}
          </a>
        );
      })}
    </nav>
  );
}

function GridBackdrop() {
  return (
    <div
      aria-hidden
      className="pointer-events-none absolute inset-x-0 top-0 -z-0 h-[900px]"
      style={{
        backgroundImage:
          "linear-gradient(to right, rgba(255,255,255,0.04) 1px, transparent 1px), linear-gradient(to bottom, rgba(255,255,255,0.04) 1px, transparent 1px)",
        backgroundSize: "48px 48px",
        maskImage: "radial-gradient(ellipse 70% 55% at 50% 0%, #000 30%, transparent 80%)",
      }}
    />
  );
}

function Hero() {
  const { content } = useSite();
  return (
    <section id="preview" className="relative z-10 mx-auto w-full max-w-[1180px] px-6 pb-24 pt-16 lg:pt-24">
      <motion.div variants={fadeUp} initial="hidden" animate="show" className="max-w-[46rem]">
        <p className="font-mono text-[11px] uppercase tracking-[0.22em] text-white/45">
          {content.hero.eyebrow}
        </p>
        <h1 className="mt-5 text-[clamp(2.1rem,4.2vw,3.25rem)] font-semibold leading-[1.08] tracking-[-0.02em]">
          <span className="block">{content.hero.headline[0]}</span>
          <span className="mt-4 block text-[clamp(1.15rem,2vw,1.45rem)] font-medium leading-snug text-[#67E8F9]">
            {content.hero.headline[1]}
          </span>
        </h1>
        <p className="mt-5 max-w-[46rem] text-[15.5px] leading-[1.65] text-white/65">
          {content.hero.body}
        </p>

        <div className="mt-7 max-w-[520px]">
          <InstallSnippet command={content.brand.installCommand} />
        </div>

        <div className="mt-5 flex flex-wrap items-center gap-x-5 gap-y-2 text-[12.5px] text-white/55">
          {content.hero.metaRow.map((m) => (
            <span key={m.key} className="flex items-center gap-2">
              <span className="h-1 w-1 rounded-full bg-white/30" />
              <span className="font-mono text-white/40">{m.key}</span>
              <span>{m.value}</span>
            </span>
          ))}
        </div>
      </motion.div>

      <motion.div
        variants={fadeUp}
        initial="hidden"
        animate="show"
        transition={{ delay: 0.08 }}
        className="mt-10"
      >
        <JobConsole />
        <p className="mt-3 font-mono text-[11px] text-white/40">{content.preview.hint}</p>
      </motion.div>
    </section>
  );
}

function InstallSnippet({ command }: { command: string }) {
  const { content } = useSite();
  const [copied, setCopied] = useState(false);
  const copy = () => {
    if (typeof navigator === "undefined") return;
    navigator.clipboard?.writeText(command).catch(() => {});
    setCopied(true);
    setTimeout(() => setCopied(false), 1400);
  };
  return (
    <div className="flex items-center gap-3 rounded-lg border border-white/[0.08] bg-[#03040A] px-4 py-3 font-mono text-[13px]">
      <span className="text-white/30">$</span>
      <span className="flex-1 truncate text-white/90">{command}</span>
      <button
        type="button"
        onClick={copy}
        className="rounded-md border border-white/10 bg-white/[0.04] px-2 py-1 text-[10px] uppercase tracking-[0.18em] text-white/65 transition-colors hover:border-[#22D3EE]/50 hover:text-[#67E8F9]"
      >
        {copied ? content.ui.copied : content.ui.copy}
      </button>
    </div>
  );
}

/** Category badge fill — the product's Ours / Other / Noise palette. */
const CATEGORY_FILL: Record<PanelJob["category"], string> = {
  ours: colors.status.running.fg,
  other: "#60A5FA",
  noise: colors.status.disabled.fg,
};

/** Active status-tab underline, one colour per launchd status. */
const TAB_COLOR: Record<string, string> = {
  all: "rgba(244,246,251,0.55)",
  running: colors.status.running.fg,
  scheduled: colors.status.scheduled.fg,
  completed: colors.status.completed.fg,
  stopped: colors.status.stopped.fg,
  error: colors.status.error.fg,
  offline: colors.status.offline.fg,
  disabled: colors.status.disabled.fg,
};

const BADGE_TONE: Record<string, string> = {
  alerts: "border-[#34D399]/30 bg-[#34D399]/10 text-[#34D399]",
  inventory: "border-[#60A5FA]/30 bg-[#60A5FA]/10 text-[#93C5FD]",
  write: "border-[#22D3EE]/30 bg-[#22D3EE]/10 text-[#67E8F9]",
  url: "border-white/10 bg-white/[0.03] text-white/50",
};

/** Tab order, mirrors the dashboard's `STATUS_KEYS` (minus "all", which leads). */
const PANEL_STATUS_ORDER: readonly ServiceStatus[] = [
  "running",
  "scheduled",
  "completed",
  "stopped",
  "error",
  "offline",
  "disabled",
];

type PreviewScene = "views" | "metrics" | "telegram";
type PreviewCategory = "all" | "ours" | "noise" | "other";

/**
 * The hero panel is a live preview of the product, not a screenshot.
 * Three scenes match the three things worth seeing: Ours / Other / Noise,
 * the Prometheus `/metrics` scrape, and a Telegram alert. Category chips
 * filter the job table the way the dashboard does. Default is Ours.
 *
 * Machine facts come from `content/fixtures.ts`; every string is localised.
 */
function JobConsole() {
  const { content } = useSite();
  const p = content.livePanel;
  const preview = content.preview;
  const [scene, setScene] = useState<PreviewScene>("views");
  const [category, setCategory] = useState<PreviewCategory>("ours");

  const categories: { key: PreviewCategory; label: string; count: number; hidden: boolean }[] = [
    { key: "all", label: p.filters.all, count: p.counts.categories.all, hidden: false },
    { key: "ours", label: p.filters.ours, count: p.counts.categories.ours, hidden: false },
    { key: "noise", label: p.filters.noise, count: p.counts.categories.noise, hidden: category !== "noise" },
    { key: "other", label: p.filters.other, count: p.counts.categories.other, hidden: false },
  ];
  const statusTabs = [
    { key: "all", label: p.filters.all, count: p.counts.statuses.all, active: category !== "noise" },
    ...PANEL_STATUS_ORDER.map((status) => ({
      key: status as string,
      label: content.statusMeta[status],
      count: p.counts.statuses[status],
      active: false,
    })),
  ];
  const badges = [
    { icon: "🔔", text: p.badges.alerts, tone: "alerts" },
    { icon: "📦", text: p.badges.inventory, tone: "inventory" },
    { icon: "✎", text: p.badges.writeMode, tone: "write" },
    { icon: "", text: p.badges.url, tone: "url" },
  ];
  const scenes: { key: PreviewScene; label: string }[] = [
    { key: "views", label: preview.views },
    { key: "metrics", label: preview.metrics },
    { key: "telegram", label: preview.telegram },
  ];
  const groups = p.groups
    .map((g) => ({
      ...g,
      rows: g.rows.filter((row) => category === "all" || row.category === category),
    }))
    .filter((g) => g.rows.length > 0);
  const noiseOn = category === "noise";

  return (
    <Card
      tone="raised"
      data-preview=""
      data-scene={scene}
      data-category={category}
      className="overflow-hidden border-white/[0.08] p-0"
    >
      <header className="flex flex-wrap items-center justify-between gap-3 border-b border-white/[0.06] px-4 py-2.5">
        <div className="flex min-w-0 items-center gap-2.5">
          <span className="flex shrink-0 gap-1">
            <span className="h-2 w-2 rounded-full bg-[#FF5F57]" />
            <span className="h-2 w-2 rounded-full bg-[#FEBC2E]" />
            <span className="h-2 w-2 rounded-full bg-[#28C840]" />
          </span>
          <span className="truncate font-mono text-[11px] text-white/55">{p.title}</span>
        </div>
        <div className="flex shrink-0 items-center gap-2">
          <div className="flex rounded-md border border-white/10 bg-[#03040A] p-0.5" role="tablist" aria-label={preview.views}>
            {scenes.map((s) => (
              <button
                key={s.key}
                type="button"
                role="tab"
                aria-selected={scene === s.key}
                data-scene-tab={s.key}
                onClick={() => setScene(s.key)}
                className={cn(
                  "rounded px-2 py-0.5 font-mono text-[10px] transition-colors",
                  scene === s.key ? "bg-[#22D3EE]/15 text-[#67E8F9]" : "text-white/45 hover:text-white/80",
                )}
              >
                {s.label}
              </button>
            ))}
          </div>
          <span className="hidden items-center gap-1.5 font-mono text-[10px] uppercase tracking-[0.2em] text-[#34D399] sm:flex">
            <span className="relative inline-flex h-1.5 w-1.5">
              <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-[#34D399] opacity-70" />
              <span className="relative inline-flex h-1.5 w-1.5 rounded-full bg-[#34D399]" />
            </span>
            {p.liveBadge}
          </span>
        </div>
      </header>

      {scene === "views" ? (
        <>
          <div className="border-b border-white/[0.05] px-4 py-3">
            <div className="flex items-start justify-between gap-3">
              <div className="min-w-0">
                <div className="truncate text-[15px] font-semibold tracking-tight text-white/95">
                  {content.brand.name}
                </div>
                <div className="truncate text-[11px] text-white/45">{p.appTagline}</div>
              </div>
              <div className="flex shrink-0 items-center gap-0.5">
                {["System", "Light", "Dark"].map((label) => (
                  <span
                    key={label}
                    className={cn(
                      "rounded-full px-1.5 py-0.5 font-mono text-[9px]",
                      label === "Dark" ? "bg-white/[0.06] text-white/70" : "text-white/35",
                    )}
                  >
                    {label}
                  </span>
                ))}
                <span className="ml-1 rounded-md border border-white/10 bg-white/[0.04] px-1.5 py-0.5 font-mono text-[9.5px] text-white/60">
                  ⚙ {content.ui.settings}
                </span>
              </div>
            </div>
            <div className="mt-2.5 flex flex-wrap items-center gap-1.5">
              {badges.map((b) => (
                <span
                  key={b.text}
                  className={cn(
                    "inline-flex items-center gap-1 rounded-full border px-2 py-0.5 font-mono text-[9.5px]",
                    BADGE_TONE[b.tone],
                  )}
                >
                  {b.icon ? <span aria-hidden>{b.icon}</span> : null}
                  {b.text}
                </span>
              ))}
            </div>
          </div>

          <div className="border-b border-white/[0.05] px-4 py-2">
            <div className="flex items-center gap-2 rounded-md border border-white/[0.08] bg-[#03040A] px-2.5 py-1.5">
              <span aria-hidden className="text-[11px] text-white/30">⌕</span>
              <span className="text-[11px] text-white/35">{p.searchPlaceholder}</span>
            </div>
          </div>

          <div className="flex flex-col gap-2 border-b border-white/[0.05] px-4 py-2.5">
            <div className="flex flex-wrap items-center justify-between gap-x-3 gap-y-2">
              <div className="flex flex-wrap gap-1.5" role="group" aria-label={preview.views}>
                {categories.map((c) => {
                  const active = category === c.key;
                  return (
                    <button
                      key={c.key}
                      type="button"
                      aria-pressed={active}
                      data-category-tab={c.key}
                      onClick={() => setCategory(c.key)}
                      className={cn(
                        "inline-flex items-center gap-1 rounded-full border px-2 py-0.5 font-mono text-[10px] transition-colors",
                        active
                          ? "border-[#22D3EE]/40 bg-[#22D3EE]/10 font-semibold text-[#67E8F9]"
                          : "border-white/10 text-white/55 hover:text-white/80",
                        c.key === "noise" && !active && "border-dashed opacity-80",
                      )}
                    >
                      {c.label}
                      <span className={active ? "text-[#67E8F9]/70" : "text-white/35"}>
                        {c.count}
                        {c.hidden ? p.hiddenSuffix : ""}
                      </span>
                    </button>
                  );
                })}
              </div>
              <button
                type="button"
                aria-pressed={noiseOn}
                onClick={() => setCategory(noiseOn ? "ours" : "noise")}
                className="flex items-center gap-1.5 text-[10px] text-white/40 hover:text-white/70"
              >
                {p.noiseToggle}
                <span
                  className={cn(
                    "relative inline-flex h-[14px] w-[26px] rounded-full border",
                    noiseOn ? "border-[#22D3EE]/40 bg-[#22D3EE]/20" : "border-white/10 bg-white/[0.06]",
                  )}
                >
                  <span
                    className={cn(
                      "absolute top-[2px] h-[8px] w-[8px] rounded-full",
                      noiseOn ? "left-[14px] bg-[#67E8F9]" : "left-[2px] bg-white/50",
                    )}
                  />
                </span>
              </button>
            </div>
            <div className="flex flex-wrap items-center gap-x-2.5 gap-y-1">
              {statusTabs.map((t) => (
                <span
                  key={t.key}
                  className={cn(
                    "border-b-2 pb-0.5 font-mono text-[10.5px]",
                    t.active ? "font-semibold text-white/90" : "text-white/40",
                  )}
                  style={{ borderBottomColor: t.active ? TAB_COLOR[t.key] : "transparent" }}
                >
                  {t.label} ({t.count})
                </span>
              ))}
            </div>
          </div>

          <div>
            {category === "noise" ? (
              <table className="w-full border-collapse text-left">
                <thead>
                  <tr className="border-b border-white/[0.06]">
                    <th className="w-7 px-3 py-1.5" />
                    {p.columns.map((c, i) => (
                      <th
                        key={c}
                        className={cn(
                          "whitespace-nowrap px-2 py-1.5 font-mono text-[9px] font-semibold uppercase tracking-[0.12em] text-white/40",
                          i === p.columns.length - 1 && "pr-3 text-right",
                        )}
                      >
                        {c}
                      </th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  <tr className="border-b border-white/[0.05] bg-white/[0.025]">
                    <td colSpan={p.columns.length + 1} className="px-3 py-1.5">
                      <div className="flex flex-wrap items-baseline gap-x-2 gap-y-0.5">
                        <span className="text-[11px] font-medium text-white/70">{preview.noiseHeading}</span>
                        <span className="font-mono text-[10px] text-white/35">{preview.noiseNote}</span>
                      </div>
                    </td>
                  </tr>
                  {noiseJobs.map((job) => (
                    <DemoRow key={job.label} job={job} panel={p} muted />
                  ))}
                </tbody>
              </table>
            ) : (
              <JobTable panel={p} groups={groups} />
            )}
          </div>

          <footer className="flex items-center justify-between border-t border-white/[0.06] bg-white/[0.015] px-4 py-2 font-mono text-[10px] uppercase tracking-[0.18em] text-white/40">
            <span>{p.events}</span>
            <span>{p.streamType}</span>
            <span>{p.push}</span>
          </footer>
        </>
      ) : null}

      {scene === "metrics" ? (
        <div className="min-h-[280px] bg-[#03040A]">
          <div className="border-b border-white/[0.06] px-4 py-2 font-mono text-[10.5px] uppercase tracking-[0.16em] text-white/40">
            {content.metrics.sampleLabel}
          </div>
          <pre className="overflow-x-auto px-4 py-3.5 font-mono text-[12px] leading-[1.75] text-white/75">
            {content.metrics.sample.map((line) => (
              <div key={line} className={cn(line.startsWith("#") && "text-white/35")}>
                {line}
              </div>
            ))}
          </pre>
          <div className="border-t border-white/[0.06] px-4 py-2 font-mono text-[10.5px] uppercase tracking-[0.16em] text-white/40">
            {content.metrics.queryLabel}
          </div>
          <pre className="overflow-x-auto px-4 py-3 font-mono text-[12px] leading-[1.75] text-[#C4B5FD]">
            {content.metrics.query.map((line) => (
              <div key={line} className={cn(line.startsWith("#") && "text-white/35")}>
                {line}
              </div>
            ))}
          </pre>
        </div>
      ) : null}

      {scene === "telegram" ? (
        <div className="min-h-[280px] bg-[#05070D] p-5">
          <div className="mx-auto max-w-[440px] rounded-xl border border-white/[0.08] bg-[#17212B] p-4 shadow-[0_12px_40px_rgba(0,0,0,0.35)]">
            <div className="flex items-center gap-2">
              <span className="flex h-8 w-8 items-center justify-center rounded-full bg-[#229ED9] font-mono text-[13px] text-white">
                ✈
              </span>
              <div>
                <div className="text-[13px] font-medium text-white/90">{content.alerts.channel}</div>
                <div className="font-mono text-[10px] text-white/40">{content.brand.name}</div>
              </div>
            </div>
            <div className="mt-3 space-y-0.5 rounded-lg bg-[#0E1621] px-3 py-2.5 font-mono text-[12.5px] leading-[1.7] text-white/85">
              {content.alerts.message.map((line) => (
                <div key={line}>{line}</div>
              ))}
            </div>
          </div>
          <ul className="mx-auto mt-4 flex max-w-[440px] flex-wrap gap-1.5">
            {content.alerts.rules.map((rule) => (
              <li
                key={rule.kind}
                className="rounded-full border border-white/10 px-2 py-0.5 font-mono text-[10px] text-white/55"
              >
                {rule.kind}
              </li>
            ))}
          </ul>
        </div>
      ) : null}
    </Card>
  );
}

function JobTable({
  panel,
  groups,
}: {
  panel: Content["livePanel"];
  groups: readonly { key: string; health: { running: number; error: number; disabled: number }; total: number; rows: readonly PanelJob[] }[];
}) {
  return (
    <table className="w-full border-collapse text-left">
      <thead>
        <tr className="border-b border-white/[0.06]">
          <th className="w-7 px-3 py-1.5" />
          {panel.columns.map((c, i) => (
            <th
              key={c}
              className={cn(
                "whitespace-nowrap px-2 py-1.5 font-mono text-[9px] font-semibold uppercase tracking-[0.12em] text-white/40",
                i === panel.columns.length - 1 && "pr-3 text-right",
              )}
            >
              {c}
            </th>
          ))}
        </tr>
      </thead>
      <tbody>
        {groups.map((g) => (
          <Fragment key={g.key}>
            <tr className="border-b border-white/[0.05] bg-white/[0.025]">
              <td colSpan={panel.columns.length + 1} className="px-3 py-1.5">
                <div className="flex items-center gap-2">
                  <span className="text-[11px] font-medium text-white/85">
                    {panel.groupNames[g.key as keyof typeof panel.groupNames]}
                  </span>
                  <span className="flex items-center gap-2 font-mono text-[9.5px] text-white/45">
                    <span className="flex items-center gap-1">
                      <span className="h-1.5 w-1.5 rounded-full" style={{ backgroundColor: colors.status.running.fg }} />
                      {g.health.running}
                    </span>
                    <span className="flex items-center gap-1">
                      <span className="h-1.5 w-1.5 rounded-full" style={{ backgroundColor: colors.status.error.fg }} />
                      {g.health.error}
                    </span>
                    <span className="flex items-center gap-1">
                      <span className="h-1.5 w-1.5 rounded-full" style={{ backgroundColor: colors.status.disabled.fg }} />
                      {g.health.disabled}
                    </span>
                    <span className="text-white/30">
                      {g.total} {panel.jobsSuffix}
                    </span>
                  </span>
                </div>
              </td>
            </tr>
            {g.rows.map((job) => (
              <DemoRow key={job.label} job={job} panel={panel} />
            ))}
          </Fragment>
        ))}
      </tbody>
    </table>
  );
}

/** One job row, matching the dashboard's row vocabulary. */
function DemoRow({
  job,
  panel,
  muted = false,
}: {
  job: PanelJob;
  panel: Content["livePanel"];
  muted?: boolean;
}) {
  const action =
    job.status === "running"
      ? panel.actions.restart
      : job.status === "disabled"
        ? panel.actions.enable
        : panel.actions.start;
  return (
    <tr className={cn("border-b border-white/[0.03] transition-colors hover:bg-white/[0.02]", muted && "opacity-60")}>
      <td className="px-3 py-1.5 align-middle">
        <StatusDot status={job.status} pulse={job.status === "running"} />
      </td>
      <td className="px-2 py-1.5 align-middle">
        <div className="flex flex-wrap items-center gap-x-1 gap-y-0.5">
          <span className="font-mono text-[11px] text-white/85">{job.label}</span>
          <span
            className="whitespace-nowrap rounded-[3px] px-1 py-px font-mono text-[8.5px] font-semibold uppercase tracking-[0.08em] text-white/90"
            style={{ backgroundColor: CATEGORY_FILL[job.category] }}
          >
            {panel.filters[job.category]}
          </span>
          <span className="whitespace-nowrap rounded-[3px] border border-dashed border-white/15 px-1 py-px font-mono text-[8.5px] text-white/45">
            {panel.sourceLabels[job.source]}
          </span>
          {job.when ? (
            <span className="whitespace-nowrap rounded-[3px] border border-white/15 px-1 py-px font-mono text-[8.5px] text-white/45">
              {panel.nextRunPrefix} {job.when}
            </span>
          ) : null}
          {job.retired ? (
            <span className="whitespace-nowrap rounded-[3px] border border-white/15 px-1 py-px font-mono text-[8.5px] text-white/45">
              {panel.retiredLabel}
            </span>
          ) : null}
          {job.churn === "churn" ? (
            <span className="whitespace-nowrap rounded-[3px] border border-[#F59E0B]/50 px-1 py-px font-mono text-[8.5px] text-[#F59E0B]">
              {panel.churnLabel}
            </span>
          ) : null}
          {job.churn === "storm" ? (
            <span className="whitespace-nowrap rounded-[3px] border border-[#F87171]/60 px-1 py-px font-mono text-[8.5px] font-semibold text-[#F87171]">
              {panel.restartLoopLabel}
            </span>
          ) : null}
        </div>
      </td>
      <td className="whitespace-nowrap px-2 py-1.5 font-mono text-[10.5px] text-white/70">
        {job.pid ?? "—"}
        {job.uptime ? <span className="ml-1.5 text-[9px] text-white/35">{job.uptime}</span> : null}
      </td>
      <td className="px-2 py-1.5 font-mono text-[10.5px] text-white/60">
        {job.status === "disabled" ? "—" : job.exit}
      </td>
      <td className="px-2 py-1.5 font-mono text-[10.5px] text-white/60">
        {job.runs}
        {job.keepAlive ? (
          <span className="ml-1 text-[#F59E0B]" title="KeepAlive">
            ↻
          </span>
        ) : null}
      </td>
      <td className="px-2 py-1.5 pr-3">
        <div className="flex items-center justify-end gap-1">
          <span
            aria-hidden={!job.alert}
            title={job.alert ? panel.alertsOn : panel.alertsOff}
            className="text-[10px]"
            style={{ color: job.alert ? "#F59E0B" : "rgba(255,255,255,0.2)" }}
          >
            {job.alert ? "🔔" : "🔕"}
          </span>
          <span className="whitespace-nowrap rounded border border-white/10 bg-white/[0.03] px-1.5 py-0.5 font-mono text-[9px] text-white/60">
            {action}
          </span>
          <span className="whitespace-nowrap rounded border border-white/10 bg-white/[0.03] px-1.5 py-0.5 font-mono text-[9px] text-white/60">
            {panel.actions.logs}
          </span>
        </div>
      </td>
    </tr>
  );
}

function StatusReference() {
  const { content } = useSite();
  const { statusSection } = content;
  return (
    <section id="features" className="relative z-10 mx-auto w-full max-w-[1180px] px-6 py-24">
      <SectionHeader
        eyebrow={statusSection.eyebrow}
        title={statusSection.title}
        body={statusSection.body}
      />
      <div className="mt-12 grid grid-cols-1 gap-px overflow-hidden rounded-xl border border-white/[0.07] bg-white/[0.04] sm:grid-cols-2 lg:grid-cols-3">
        {content.statuses.map(({ status, rule }) => {
          const meta = colors.status[status];
          return (
            <div key={status} className="bg-[#05070D] p-6">
              <div className="flex items-center gap-2.5">
                <StatusDot status={status as ServiceStatus} />
                <span
                  className="font-mono text-[12px] uppercase tracking-[0.18em]"
                  style={{ color: meta.fg }}
                >
                  {content.statusMeta[status]}
                </span>
              </div>
              <p className="mt-4 font-mono text-[12px] leading-[1.6] text-white/70">
                {rule}
              </p>
            </div>
          );
        })}
      </div>
    </section>
  );
}

function WhySection() {
  const { content } = useSite();
  const { why } = content;
  return (
    <section id="why" className="relative z-10 mx-auto w-full max-w-[1180px] px-6 py-24">
      <SectionHeader eyebrow={why.eyebrow} title={why.headline} />
      <div className="mt-10 grid grid-cols-1 gap-8 lg:grid-cols-[1.15fr_1fr] lg:gap-14">
        <div className="space-y-5">
          {why.paragraphs.map((p, i) => (
            <p key={i} className="text-[15px] leading-[1.7] text-white/60">
              {p}
            </p>
          ))}
        </div>
        <div className="grid grid-cols-1 gap-4">
          {[why.contrast.before, why.contrast.after].map((col, idx) => (
            <Card
              key={col.label}
              className={cn(
                "border-white/[0.07] p-5",
                idx === 0 ? "bg-white/[0.015]" : "bg-[#22D3EE]/[0.04]",
              )}
            >
              <span
                className={cn(
                  "font-mono text-[10.5px] uppercase tracking-[0.18em]",
                  idx === 0 ? "text-white/40" : "text-[#67E8F9]",
                )}
              >
                {col.label}
              </span>
              <ul className="mt-4 space-y-2.5">
                {col.bullets.map((b) => (
                  <li key={b} className="flex gap-2.5 text-[13.5px] leading-[1.6] text-white/65">
                    <span className={idx === 0 ? "text-white/30" : "text-[#34D399]"} aria-hidden>
                      {idx === 0 ? "×" : "✓"}
                    </span>
                    <span>{b}</span>
                  </li>
                ))}
              </ul>
            </Card>
          ))}
        </div>
      </div>
    </section>
  );
}

function ForkAdditions() {
  const { content } = useSite();
  const { forkSection } = content;
  const accents = {
    brand: { fg: "#67E8F9", border: "border-[#22D3EE]/25", tint: "bg-[#22D3EE]/[0.05]" },
    accent: { fg: "#C4B5FD", border: "border-[#A78BFA]/25", tint: "bg-[#A78BFA]/[0.05]" },
    warning: { fg: "#FCD34D", border: "border-[#F59E0B]/25", tint: "bg-[#F59E0B]/[0.05]" },
  } as const;

  return (
    <section id="ours" className="relative z-10 mx-auto w-full max-w-[1180px] px-6 py-24">
      <SectionHeader
        eyebrow={forkSection.eyebrow}
        title={forkSection.title}
        body={forkSection.body}
      />
      <div className="mt-12 grid grid-cols-1 gap-4 md:grid-cols-2">
        {content.forkAdditions.map((f) => {
          const a = accents[f.accent ?? "brand"];
          return (
            <Card key={f.index} className={cn("p-6", a.border, a.tint)}>
              <span className="font-mono text-[10.5px] uppercase tracking-[0.2em]" style={{ color: a.fg }}>
                {f.index}
              </span>
              <h3 className="mt-3 text-[16.5px] font-semibold leading-snug tracking-[-0.01em] text-white/95">
                {f.title}
              </h3>
              <p className="mt-3 text-[13.5px] leading-[1.7] text-white/60">{f.body}</p>
            </Card>
          );
        })}
      </div>

      <div className="mt-8 overflow-hidden rounded-xl border border-white/[0.07]">
        <div className="border-b border-white/[0.06] bg-white/[0.025] px-6 py-2.5 font-mono text-[10.5px] uppercase tracking-[0.16em] text-white/40">
          {forkSection.upstreamLabel}
        </div>
        <div className="grid grid-cols-1 gap-px bg-white/[0.05] sm:grid-cols-2 lg:grid-cols-3">
          {forkSection.keepers.map((k) => (
            <div key={k.name} className="bg-[#05070D] p-5">
              <span className="font-mono text-[12.5px] tracking-tight text-white/85">{k.name}</span>
              <p className="mt-2 text-[12.5px] leading-[1.6] text-white/50">{k.body}</p>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}

function CodeBlock({ lines, label }: { lines: readonly string[]; label?: string }) {
  return (
    <div className="overflow-hidden rounded-xl border border-white/[0.07] bg-[#03040A]">
      {label ? (
        <div className="border-b border-white/[0.06] bg-white/[0.025] px-4 py-2 font-mono text-[10.5px] uppercase tracking-[0.16em] text-white/40">
          {label}
        </div>
      ) : null}
      <pre className="overflow-x-auto px-4 py-3.5 font-mono text-[11.5px] leading-[1.75] text-white/70">
        {lines.map((l) => (
          <div key={l} className={cn(l.startsWith("#") && "text-white/35")}>
            {l || "\u00A0"}
          </div>
        ))}
      </pre>
    </div>
  );
}

function MetricsSection() {
  const { content } = useSite();
  const { metrics } = content;
  return (
    <section id="metrics" className="relative z-10 mx-auto w-full max-w-[1180px] px-6 py-24">
      <SectionHeader eyebrow={metrics.eyebrow} title={metrics.headline} body={metrics.body} />
      <div className="mt-12 grid grid-cols-1 gap-4 lg:grid-cols-[1.35fr_1fr]">
        <CodeBlock lines={metrics.sample} label={metrics.sampleLabel} />
        <div className="space-y-4">
          <CodeBlock lines={metrics.query} label={metrics.queryLabel} />
          <Card className="border-white/[0.07] bg-white/[0.02] p-5">
            <p className="text-[13px] leading-[1.7] text-white/60">
              {metrics.footnoteBefore}
              <span className="font-mono text-white/80">{metrics.footnoteCode}</span>
              {metrics.footnoteAfter}
            </p>
          </Card>
        </div>
      </div>
    </section>
  );
}

function AlertsSection() {
  const { content } = useSite();
  const { alerts } = content;
  return (
    <section id="alerts" className="relative z-10 mx-auto w-full max-w-[1180px] px-6 py-24">
      <SectionHeader eyebrow={alerts.eyebrow} title={alerts.headline} body={alerts.body} />
      <div className="mt-12 grid grid-cols-1 gap-4 lg:grid-cols-[1fr_1.1fr]">
        <Card tone="raised" className="border-white/[0.08] p-5">
          <div className="flex items-center gap-2">
            <span className="h-6 w-6 rounded-full bg-[#229ED9]/20 text-center font-mono text-[11px] leading-6 text-[#67E8F9]">
              ✈
            </span>
            <span className="font-mono text-[11px] uppercase tracking-[0.16em] text-white/45">
              {alerts.channel}
            </span>
          </div>
          <div className="mt-4 space-y-1 rounded-lg border border-white/[0.07] bg-[#05070D] p-4 font-mono text-[12px] leading-[1.8] text-white/75">
            {alerts.message.map((line) => (
              <div key={line}>{line}</div>
            ))}
          </div>
        </Card>
        <div className="overflow-hidden rounded-xl border border-white/[0.07]">
          <div className="grid grid-cols-[1.4fr_1fr] border-b border-white/[0.06] bg-white/[0.025] px-5 py-2.5 font-mono text-[10.5px] uppercase tracking-[0.16em] text-white/40">
            <span>{alerts.triggerHeader}</span>
            <span>{alerts.kindHeader}</span>
          </div>
          {alerts.rules.map((r, i) => (
            <div
              key={r.kind}
              className={cn(
                "grid grid-cols-[1.4fr_1fr] items-baseline px-5 py-3",
                i > 0 && "border-t border-white/[0.04]",
              )}
            >
              <span className="text-[13px] text-white/70">{r.trigger}</span>
              <span className="font-mono text-[12px] text-[#67E8F9]">{r.kind}</span>
            </div>
          ))}
          <div className="border-t border-white/[0.06] bg-white/[0.015] px-5 py-3 text-[12.5px] leading-[1.6] text-white/50">
            {alerts.footer}
          </div>
        </div>
      </div>
    </section>
  );
}

function ApiSurface() {
  const { content } = useSite();
  const { api } = content;
  return (
    <section id="how" className="relative z-10 mx-auto w-full max-w-[1180px] px-6 py-24">
      <SectionHeader eyebrow={api.eyebrow} title={api.title} body={api.body} />
      <div className="mt-12 overflow-hidden rounded-xl border border-white/[0.07] bg-[#03040A]">
        <div className="grid grid-cols-[5.5rem_1fr_1.4fr] gap-0 border-b border-white/[0.06] bg-white/[0.025] px-6 py-2.5 font-mono text-[10.5px] uppercase tracking-[0.16em] text-white/40">
          <span>{api.verb}</span>
          <span>{api.path}</span>
          <span>{api.description}</span>
        </div>
        {api.endpoints.map((e, i) => (
          <div
            key={e.path + e.method}
            className={cn(
              "grid grid-cols-[5.5rem_1fr_1.4fr] items-baseline gap-0 px-6 py-3",
              i > 0 && "border-t border-white/[0.04]",
            )}
          >
            <span
              className={cn(
                "font-mono text-[11px] uppercase tracking-[0.14em]",
                e.method === "GET" ? "text-[#67E8F9]" : "text-[#A78BFA]",
              )}
            >
              {e.method}
            </span>
            <span className="font-mono text-[12.5px] text-white/85">{e.path}</span>
            <span className="text-[13px] text-white/55">{e.note}</span>
          </div>
        ))}
      </div>
    </section>
  );
}

function SectionHeader({
  eyebrow,
  title,
  body,
}: {
  eyebrow: string;
  title: string;
  body?: string;
}) {
  return (
    <div className="grid grid-cols-1 gap-6 lg:grid-cols-12">
      <p className="font-mono text-[11px] uppercase tracking-[0.22em] text-white/40 lg:col-span-2 lg:pt-2">
        {eyebrow}
      </p>
      <div className="lg:col-span-10">
        <h2 className="max-w-[26ch] text-[clamp(1.75rem,3vw,2.5rem)] font-semibold leading-[1.1] tracking-[-0.02em]">
          {title}
        </h2>
        {body ? (
          <p className="mt-5 max-w-[68ch] text-[15.5px] leading-[1.65] text-white/60">
            {body}
          </p>
        ) : null}
      </div>
    </div>
  );
}

function FaqSection() {
  const { content } = useSite();
  const { faqSection } = content;
  return (
    <section id="faq" className="relative z-10 mx-auto w-full max-w-[1180px] px-6 py-24">
      <SectionHeader eyebrow={faqSection.eyebrow} title={faqSection.title} body={faqSection.body} />
      <div className="mt-12 grid grid-cols-1 gap-4 md:grid-cols-2">
        {content.faqs.map((f) => (
          <Card key={f.question} className="border-white/[0.07] bg-white/[0.02] p-6">
            <h3 className="text-[15px] font-semibold leading-snug text-white/90">
              {f.question}
            </h3>
            <p className="mt-3 text-[13.5px] leading-[1.7] text-white/60">{f.answer}</p>
          </Card>
        ))}
      </div>
    </section>
  );
}

function InstallBlock() {
  const { content } = useSite();
  const { install } = content;
  return (
    <section id="install" className="relative z-10 mx-auto w-full max-w-[1180px] px-6 py-24">
      <SectionHeader eyebrow={install.eyebrow} title={install.title} body={install.body} />
      <div className="mt-12 grid grid-cols-1 gap-4 lg:grid-cols-3">
        {content.steps.map((s, i) => (
          <Card
            key={s.index}
            tone={i === 2 ? "glow" : undefined}
            className={cn("p-6", i < 2 && "border-white/[0.07] bg-white/[0.02]")}
          >
            <span
              className={cn(
                "font-mono text-[10.5px] uppercase tracking-[0.18em]",
                i === 2 ? "text-[#67E8F9]" : "text-white/40",
              )}
            >
              {s.index} · {s.title.toLowerCase()}
            </span>
            {s.command ? (
              <pre className="mt-5 overflow-x-auto rounded-md border border-white/[0.07] bg-[#03040A] px-3 py-2.5 font-mono text-[12px] text-[#67E8F9]">
                <span className="text-white/35">$ </span>
                {s.command}
              </pre>
            ) : (
              <div className="mt-5 h-[38px]" aria-hidden />
            )}
            <p className="mt-4 text-[13px] leading-[1.65] text-white/55">{s.body}</p>
          </Card>
        ))}
      </div>

      <div className="mt-8 grid grid-cols-1 gap-4 md:grid-cols-3">
        {content.stats.map((st) => (
          <div key={st.label} className="border-l border-white/[0.08] pl-4">
            <span className="font-mono text-[20px] tabular-nums text-white/90">{st.value}</span>
            <p className="mt-1 font-mono text-[11px] uppercase tracking-[0.16em] text-white/45">
              {st.label}
            </p>
            <p className="mt-1.5 text-[12.5px] text-white/45">{st.caption}</p>
          </div>
        ))}
      </div>
    </section>
  );
}

function SiteFooter() {
  const { content } = useSite();
  return (
    <footer className="relative z-10 border-t border-white/[0.06]">
      <div className="mx-auto flex w-full max-w-[1180px] flex-col items-start justify-between gap-6 px-6 py-10 md:flex-row md:items-center">
        <div className="flex items-baseline gap-2">
          <span aria-hidden className="text-[13px] text-[#67E8F9]">▸</span>
          <span className="font-mono text-[12px] text-white/40">
            {content.footer.copyline}
          </span>
        </div>
        <ul className="flex flex-wrap gap-x-6 gap-y-2 text-[13px] text-white/55">
          {content.footer.links.map((l) => (
            <li key={l.href}>
              <a href={l.href} target="_blank" rel="noreferrer" className="transition-colors hover:text-white">
                {l.label}
              </a>
            </li>
          ))}
        </ul>
      </div>
    </footer>
  );
}
