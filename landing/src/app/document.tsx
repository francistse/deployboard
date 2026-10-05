import { Space_Grotesk, JetBrains_Mono } from "next/font/google";
import { Providers } from "@/page/providers";
import type { Locale } from "@/content";

const spaceGrotesk = Space_Grotesk({
  variable: "--font-space-grotesk",
  subsets: ["latin"],
});

// The page is a dev console: launchd labels, log lines, the `/metrics` sample
// and API paths are all monospaced surfaces. This must be a real mono — it once
// pointed at a proportional face, mis-setting every `.font-mono` element and
// every <pre> on the page.
const jetbrainsMono = JetBrains_Mono({
  variable: "--font-jetbrains-mono",
  subsets: ["latin"],
});

export function Document({
  lang,
  children,
}: {
  lang: Locale;
  children: React.ReactNode;
}) {
  return (
    <html lang={lang} className={`${spaceGrotesk.variable} ${jetbrainsMono.variable}`}>
      <body className="antialiased bg-background text-foreground">
        <Providers>{children}</Providers>
      </body>
    </html>
  );
}
