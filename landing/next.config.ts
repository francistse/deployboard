import type { NextConfig } from "next";

/**
 * Static export for GitHub Pages.
 *
 * A project page lives at https://<user>.github.io/<repo>/, so assets must be
 * served from a sub-path. The deploy workflow sets NEXT_PUBLIC_BASE_PATH from
 * the repository name; for a user/organisation page (or local preview) it stays
 * empty and the site is served from the root.
 */
const basePath = process.env.NEXT_PUBLIC_BASE_PATH ?? "";

const nextConfig: NextConfig = {
  output: "export",
  images: { unoptimized: true },
  turbopack: { root: __dirname },
  basePath,
  assetPrefix: basePath || undefined,
  trailingSlash: true,
  env: { NEXT_PUBLIC_BASE_PATH: basePath },
};

export default nextConfig;
